package gotd

import (
	"context"
	"mime"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"time"

	goerrs "github.com/go-faster/errors"
	"github.com/google/uuid"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"github.com/micro/micro/v3/service/client"
	"github.com/micro/micro/v3/service/errors"
	"github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/api/proto/storage"
	"github.com/webitel/chat_manager/bot"
	"go.uber.org/multierr"
)

const defaultPartSize = 512 * 1024 // 512 kb

// getFile pumps source (external, telegram) file location into target (internal, storage) media file
func getFile(account *app, mediaFile *chat.File, inputFile tg.InputFileLocationClass) (*chat.File, error) {

	var (
		// GET
		tgapi   = account.Client.API()
		getFile = tg.UploadGetFileRequest{
			Precise:      false,
			CDNSupported: false,
			Location:     inputFile,
			Offset:       0,
			Limit:        defaultPartSize,
		}
		// SET
		stream storage.FileService_UploadFileService
		mpart  = storage.UploadFileRequest_Chunk{
			Chunk: nil,
		}
		push = storage.UploadFileRequest{
			Data: &mpart,
		}
		// CTX
		ctx  = context.Background()
		data []byte // content part
	)

loop:
	for {
		// READ
		part, err := tgapi.UploadGetFile(ctx, &getFile)
		if flood, err := tgerr.FloodWait(ctx, err); err != nil {
			if flood || tgerr.Is(err, tg.ErrTimeout) {
				continue
			}
			// return block{}, errors.Wrap(err, "get next chunk")
			return nil, err
		}
		// https://core.telegram.org/type/upload.File
		switch part := part.(type) {
		case *tg.UploadFile:
			// Advance
			data = part.Bytes
			getFile.Offset += int64(len(data)) // getFile.Limit
			if stream == nil {
				// Init target
				grpcClient := client.DefaultClient
				store := storage.NewFileService("storage", grpcClient)
				stream, err = store.UploadFile(ctx)
				if err != nil {
					return nil, err
				}
				// https://core.telegram.org/type/storage.FileType
				filename := "_2006-01-02_15-04-05" // combines media filename with the timestamp received
				switch part.Type.(type) {
				case *tg.StorageFileJpeg:
					mediaFile.Mime = "image/jpeg"
					filename = "image" + filename + ".jpg"
				case *tg.StorageFileGif:
					mediaFile.Mime = "image/gif"
					filename = "image" + filename + ".gif"
				case *tg.StorageFilePng:
					mediaFile.Mime = "image/png"
					filename = "image" + filename + ".png"
				case *tg.StorageFilePdf:
					mediaFile.Mime = "application/pdf"
					filename = "doc" + filename + ".pdf"
				case *tg.StorageFileMp3:
					mediaFile.Mime = "audio/mpeg"
					filename = "audio" + filename + ".mp3"
				case *tg.StorageFileMov:
					mediaFile.Mime = "video/quicktime"
					filename = "video" + filename + ".mov"
				case *tg.StorageFileMp4:
					mediaFile.Mime = "video/mp4"
					filename = "video" + filename + ".mp4"
				case *tg.StorageFileWebp:
					mediaFile.Mime = "image/webp"
					filename = "image" + filename + ".webp"
				// case *tg.StorageFileUnknown: // Unknown type.
				// 	mediaFile.Mime = "application/octet-stream"
				// case *tg.StorageFilePartial: // Part of a bigger file.
				// 	if mediaFile.Mime == "" {
				// 		panic("telegram/upload.getFile(*tg.StorageFilePartial)")
				// 	}
				default:
					// case *tg.StorageFileUnknown
					if mediaFile.Mime == "" {
						mediaFile.Mime = "application/octet-stream" // default
					}
					filename = "file" + filename + ".bin" // binary
				}
				// Default filename generation
				if mediaFile.Name == "" {
					mediaFile.Name = time.Now().Format(filename)
				}
				// INIT: WRITE
				err = stream.Send(&storage.UploadFileRequest{
					Data: &storage.UploadFileRequest_Metadata_{
						Metadata: &storage.UploadFileRequest_Metadata{
							DomainId: account.Gateway.DomainID(),
							MimeType: mediaFile.Mime,
							Name:     mediaFile.Name,
							Uuid:     uuid.Must(uuid.NewRandom()).String(),
						},
					},
				})
				if err != nil {
					// parse error through function to understand type of error
					return nil, bot.HandleFileUploadError(err)
				}
			}
			// WRITE
			mpart.Chunk = part.Bytes
			err = stream.Send(&push)
			if err != nil {
				if _, re := stream.CloseAndRecv(); re != nil {
					return nil, re
				}
				// parse error through function to understand type of error
				return nil, bot.HandleFileUploadError(err)
			}
			// if len(data) == 0 {
			if len(data) < getFile.Limit {
				// That was the last part !
				// Send EOF file mark !
				mpart.Chunk = nil
				_ = stream.Send(&push)
				break loop
			}
			// time.Sleep(time.Second / 2)
		case *tg.UploadFileCDNRedirect:
			return nil, &downloader.RedirectError{Redirect: part}
		default:
			// return chunk{}, errors.Errorf("unexpected type %T", chunk)
			return nil, errors.BadGateway(
				"telegram.upload.getFile.unexpected",
				"telegram/upload.getFile: unexpected result %T type",
				part,
			)
		}
	}

	// var res *storage.UploadFileResponse
	res, err := stream.CloseAndRecv()
	if err != nil {
		return nil, err
	}

	fileURI := res.FileUrl
	if path.IsAbs(fileURI) {
		// NOTE: We've got not a valid URL but filepath
		srv := account.Gateway.Internal
		hostURL, err := url.ParseRequestURI(srv.HostURL())
		if err != nil {
			panic(err)
		}
		fileURL := &url.URL{
			Scheme: hostURL.Scheme,
			Host:   hostURL.Host,
		}
		fileURL, err = fileURL.Parse(fileURI)
		if err != nil {
			panic(err)
		}
		fileURI = fileURL.String()
		res.FileUrl = fileURI
	}

	mediaFile.Id = res.FileId
	mediaFile.Url = res.FileUrl
	mediaFile.Size = res.Size
	mediaFile.Malware = res.Malware != nil && res.Malware.Found

	return mediaFile, nil
}

// FromMediaFile uploads file from storage service `media` URL using given `client` Source.
func FromMediaFile(media *chat.File, client *http.Client) message.UploadOption {
	return message.Upload(func(ctx context.Context, b message.Uploader) (_ tg.InputFileClass, re error) {

		sourceURL, err := url.ParseRequestURI(media.Url)
		if err != nil {
			return nil, goerrs.Wrapf(err, "parse url %q", media.Url)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL.String(), nil)
		if err != nil {
			return nil, goerrs.Wrap(err, "create request")
		}

		if client == nil {
			client = http.DefaultClient
		}
		rsp, err := client.Do(req)
		if err != nil {
			return nil, goerrs.Wrap(err, "get")
		}
		defer func() {
			if re != nil {
				multierr.AppendInto(&re, rsp.Body.Close())
			}
		}()
		if rsp.StatusCode >= 400 {
			return nil, goerrs.Errorf("bad code %d", rsp.StatusCode)
		}

		filename := media.Name
		if filename == "" {
			// Content-Disposition
			if disposition := rsp.Header.Get("Content-Disposition"); disposition != "" {
				if _, params, err := mime.ParseMediaType(disposition); err == nil {
					if filename = params["filename"]; filename != "" {
						// RFC 7578, Section 4.2 requires that if a filename is provided, the
						// directory path information must not be used.
						switch filename = filepath.Base(filename); filename {
						case ".", string(filepath.Separator):
							filename = "" // invalid
						}
					}
				}
			}
			if filename == "" {
				if rsp.Request.URL != nil {
					sourceURL = rsp.Request.URL
				}
				filename = path.Base(sourceURL.Path)
			}
		}
		// Resolve filename extension if omitted; AUDIO !
		if ext := filepath.Ext(filename); ext == "" {
			mediaType := rsp.Header.Get("Content-Type")
			if mediaType, _, err = mime.ParseMediaType(mediaType); err == nil {
				if ext, _ := mime.ExtensionsByType(mediaType); len(ext) != 0 {
					filename += ext[0]
				}
			}
		}

		return b.FromReader(ctx, filename, rsp.Body)
	})
}
