package vk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	vk "github.com/SevereCloud/vksdk/v2/api"
	"github.com/SevereCloud/vksdk/v2/object"
	"github.com/micro/micro/v3/service/errors"
	chat "github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/bot"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
)

const (
	provider = "vk"
	hookName = "webitel_webhook"
)

func init() {
	bot.Register(provider, NewVkBot)
}

// VK BOT chat provider
type VKBot struct {
	*bot.Gateway
	BotApi   *vk.VK
	serverId int64
	creds    *VKCreds
	contacts map[int64]*bot.Account
}

type VKCreds struct {
	// ID of group in which bot used
	GroupId string
	// secret for this group
	Secret string
	// confirmation code used for [WebHook] confirmation
	ConfirmationCode string
}

func (c *VKBot) GetServerId() int64 {
	return c.serverId
}

func (c *VKBot) GetGroupId() int64 {
	return c.serverId
}

// NewVkBot initialize new agent.profile service provider
func NewVkBot(agent *bot.Gateway, _ bot.Provider) (bot.Provider, error) {

	config := agent.Bot
	profile := config.GetMetadata()

	// Parse and validate message templates
	var err error
	agent.Template = bot.NewTemplate(provider)
	// Parse message templates
	if err = agent.Template.FromProto(
		agent.Bot.GetUpdates(),
	); err == nil {
		// Quick tests ! <nil> means default (well-known) test cases
		err = agent.Template.Test(nil)
	}
	if err != nil {
		return nil, errors.BadRequest(
			"chat.bot.vk.updates.invalid",
			err.Error(),
		)
	}

	credentials, err := makeVkCredsFromProfile(profile)
	if err != nil {
		return nil, err
	}
	var (
		botAPI     *vk.VK
		httpClient *http.Client
	)

	trace := profile["trace"]
	if on, _ := strconv.ParseBool(trace); on {
		var transport http.RoundTripper
		if httpClient != nil {
			transport = httpClient.Transport
		}
		if transport == nil {
			transport = http.DefaultTransport
		}
		transport = &bot.TransportDump{
			Transport: transport,
			WithBody:  true,
		}
		if httpClient == nil {
			httpClient = &http.Client{
				Transport: transport,
			}
		} else {
			httpClient.Transport = transport
		}
	}
	botAPI = vk.NewVK(credentials.Secret)
	if httpClient != nil {
		botAPI.Client = httpClient
	}

	return &VKBot{
		Gateway:  agent,
		BotApi:   botAPI,
		contacts: make(map[int64]*bot.Account),
		creds:    credentials,
	}, nil
}

func (c *VKBot) Close() error {
	return nil
}

// String "vk" provider's name
func (c *VKBot) String() string {
	return provider
}

func makeVkCredsFromProfile(profile map[string]string) (*VKCreds, error) {
	var res VKCreds
	if v, ok := profile["group_id"]; ok {
		res.GroupId = v
	} else {
		return nil, errors.BadRequest(
			"chat.bot.vk.group_id.required",
			"vk: group id required",
		)
	}

	if v, ok := profile["token"]; ok {
		res.Secret = v
	} else {
		return nil, errors.BadRequest(
			"chat.bot.vk.secret.required",
			"vk: secret group key required",
		)
	}

	if v, ok := profile["confirmation_code"]; ok {
		res.ConfirmationCode = v
	} else {
		return nil, errors.BadRequest(
			"chat.bot.vk.confirmation_code.required",
			"vk: server confirmation code required",
		)
	}
	return &res, nil
}

// Register VK Bot Webhook endpoint URI
func (c *VKBot) Register(ctx context.Context, callbackURL string) error {

	// // webhookInfo := tgbotapi.NewWebhookWithCert(fmt.Sprintf("%s/telegram/%v", cfg.TgWebhook, profile.Id), cfg.CertPath)
	// linkURL := strings.TrimRight(c.Gateway.Internal.URL, "/") +
	// 	("/" + c.Gateway.Profile.UrlId)
	params := vk.Params{"group_id": c.creds.GroupId, "url": callbackURL, "title": hookName}
	resp, err := c.BotApi.GroupsAddCallbackServer(params)
	if err != nil {
		c.Gateway.Log.Error().Err(err).Msg("Failed to .Register webhook")
		return err
	}
	c.serverId = int64(resp.ServerID)
	return nil
}

// Deregister Vk Bot Webhook endpoint URI
func (c *VKBot) Deregister(ctx context.Context) error {
	// POST /deleteWebhook

	params := vk.Params{"group_id": c.creds.GroupId, "server_id": c.serverId}
	res, err := c.BotApi.GroupsDeleteCallbackServer(params)
	if err != nil {
		return err
	}

	if res != http.StatusOK {
		return errors.New(
			"chat.bot.vk.deregister.error",
			fmt.Sprintf("http error code: %d", res),
			int32(res), // FIXME: 502 Bad Gateway ?
		)
	}

	return nil
}

func contactPeer(peer *chat.Account) *chat.Account {
	if peer.LastName == "" {
		peer.FirstName, peer.LastName =
			bot.FirstLastName(peer.FirstName)
	}
	return peer
}

// SendNotify implements provider.Sender interface for VK
func (c *VKBot) SendNotify(ctx context.Context, notify *bot.Update) error {

	var (
		channel = notify.Chat // recepient
		// localtime = time.Now()
		message = notify.Message

		binding map[string]string
	)

	// // TESTS

	bind := func(key, value string) {
		if binding == nil {
			binding = make(map[string]string)
		}
		binding[key] = value
	}

	var (
		sendUpdate *vk.Params
	)

	vkMessage, err := c.ConvertInternalToOutcomingMessage(notify)
	if err != nil {
		return err
	}
	if re := vkMessage.IsValid(); re != nil {
		return re
	}
	sendUpdate, err = vkMessage.Params()
	if err != nil {
		return err
	}
	bt, _ := json.Marshal(sendUpdate)
	fmt.Println(bt)
	sentMessageId, err := c.BotApi.MessagesSend(*sendUpdate)
	if err != nil {
		return err
	}

	// TARGET[chat_id]: MESSAGE[message_id]
	bind(channel.ChatID, strconv.Itoa(sentMessageId))
	if message.Id != 0 { // NOT {"type": "closed"}
		// [optional] STORE external SENT message binding
		message.Variables = binding
	}
	// +OK
	return nil
}

const (
	imageResolutionMax = 1920 * 1920
)

// WebHook implementes provider.Receiver interface for VK
func (c *VKBot) WebHook(reply http.ResponseWriter, notice *http.Request) {
	// confirmation request
	//{ "type": "confirmation", "group_id": 222705592 }

	var recvEvent VKEvent

	err := json.NewDecoder(notice.Body).Decode(&recvEvent)
	if err != nil {
		http.Error(reply, "Failed to decode vk .Update event", http.StatusBadRequest)
		c.Log.Error().Str("error", "vk.Update: "+err.Error()).Msg("VK: UPDATE")
		return // 400 Bad Request
	}

	switch notice.Method {
	case http.MethodPost:
		if notice.Body != nil {
			defer notice.Body.Close()
		}
	default:
		// Method Not Allowed !
	}

	// find out message type
	switch recvEvent.Type {
	case "message_new": // new message received

		// region INITIALIZING INTERNAL EVENT
		// constructing new [INTERNAL] message
		message := recvEvent.Object.Message
		var dialogId = strconv.FormatInt(message.PeerId, 10)

		channel, err := c.getChannel(
			notice.Context(), message,
		)
		if err != nil {
			// Failed locate chat channel !
			re := errors.FromError(err)
			if re.Code == 0 {
				re.Code = (int32)(http.StatusBadGateway)
				// HTTP 503 Bad Gateway
			}

			return // HTTP 200 OK; WITH reply error recvEvent
		}
		sendUpdate := bot.Update{
			Chat:  channel,
			Title: channel.Title,
			User:  &channel.Account,

			Message: new(chat.Message),
		}
		// endregion

		// region CONSTRUCTING NEW MESSAGE

		sendMessage := sendUpdate.Message

		coalesce := func(argv ...string) string {
			for _, s := range argv {
				if s = strings.TrimSpace(s); s != "" {
					return s
				}
			}
			return ""
		}
		// region SEND ATTACHMENTS
		if len(message.Attachments) != 0 {
			for _, attachment := range message.Attachments {
				attachmentUpdate := bot.Update{
					Chat:  channel,
					Title: channel.Title,
					User:  &channel.Account,

					Message: new(chat.Message),
				}
				attachmentMessage := attachmentUpdate.Message
				if attachmentType, ok := attachment["type"]; ok {
					switch t := attachmentType.(type) {
					case string:
						switch t {
						case "photo":

							var (
								photo Photo
							)
							err := unmarshalTo(attachment[t], &photo)
							if err != nil {
								continue
							}
							i := len(photo.Sizes) - 1 // From biggest to smallest ...
							for ; i >= 0 && (photo.Sizes[i].Height*photo.Sizes[i].Width) > imageResolutionMax; i-- {
								// omit files that are too large,
								// which will result in a download error

							}
							if i < 0 {
								i = 0
							}

							chosenPhoto := photo.Sizes[i]

							// Prepare internal message content
							attachmentMessage.Type = "file"
							attachmentMessage.File = &chat.File{
								Url:  chosenPhoto.Url, // source URL to download from ...
								Mime: "",              // autodetect on chat's service .SendMessage()
								// mime.TypeByExtension(path.Ext(image.FileName()))
								// "image/jpg",
								Name: t,
								// unknown ?
								Size: 0,
							}
							// Optional. Caption
							attachmentMessage.Text = coalesce(
								photo.Caption,
							)
						case "video":

							var (
								video Video
							)
							err := unmarshalTo(attachment[t], &video)
							if err != nil {
								continue
							}
							if video.Url != "" {
								attachmentMessage.Type = "file"
								attachmentMessage.File = &chat.File{
									Url:  video.Url, // source to download
									Mime: "",
									Name: video.Title,
									Size: 0,
								}
							} else {
								// video without url !UNSUPPORTED
								attachmentMessage.Type = "text"
								attachmentMessage.Text = "[UNSUPPORTED VIDEO]"

								//continue
							}
						case "audio":
							var (
								audio Audio
							)
							err := unmarshalTo(attachment[t], &audio)
							if err != nil {
								continue
							}
							// Prepare internal message content
							attachmentMessage.Type = "file"
							attachmentMessage.File = &chat.File{
								Url:  audio.Url, // source URL to download from ...
								Size: 0,
								Mime: "",
								Name: audio.Title,
							}
							// Optional. Caption
							attachmentMessage.Text = coalesce(
								message.Text,
							)
						case "audio_message":
							var (
								audio VoiceMessage
							)
							err := unmarshalTo(attachment[t], &audio)
							if err != nil {
								continue
							}
							// Prepare internal message content
							attachmentMessage.Type = "file"
							attachmentMessage.File = &chat.File{
								Url:  coalesce(audio.LinkMP3, audio.LinkOGG), // source URL to download from ...
								Size: 0,
								Mime: "",
								Name: t,
							}
							// Optional. Caption
							attachmentMessage.Text = coalesce(
								message.Text,
							)
						case "doc":
							var (
								doc Document
							)
							err := unmarshalTo(attachment[t], &doc)
							if err != nil {
								continue
							}
							// Prepare internal message content
							attachmentMessage.Type = "file"
							attachmentMessage.File = &chat.File{
								Url:  doc.Url, // source to download
								Mime: doc.Extension,
								Name: doc.Title,
								Size: doc.Size,
							}
							// Optional. Caption
							attachmentMessage.Text = coalesce(
								message.Text,
							)

						//case "link":
						case "sticker":
							var (
								sticker Sticker
							)
							err := unmarshalTo(attachment[t], &sticker)
							if err != nil {
								continue
							}
							i := len(sticker.Images) - 1 // From biggest to smallest ...
							for ; i >= 0 && (sticker.Images[i].Height*sticker.Images[i].Width) > imageResolutionMax; i-- {
								// omit files that are too large,
								// which will result in a download error

							}
							if i < 0 {
								i = 0
							}

							chosenPhoto := sticker.Images[i]
							// Prepare internal message content
							attachmentMessage.Type = "file"
							attachmentMessage.File = &chat.File{
								Url:  chosenPhoto.Url, // source to download
								Mime: "",
								Name: t,
								Size: 0,
							}
							// Optional. Caption

						}
					}

				}
				attachmentMessage.Variables = map[string]string{
					dialogId: dialogId,
					// "chat_id":    chatID,
					// "message_id": strconv.Itoa(recvEvent.MessageID),
				}
				//if channel.IsNew() { // && contact.Username != "" {
				//	attachmentMessage.Variables["username"] = sender. // contact.Username
				//}

				err = c.Gateway.Read(notice.Context(), &attachmentUpdate)

				if err != nil {

					code := http.StatusInternalServerError
					http.Error(reply, "Failed to forward .Update recvEvent", code)
					return // 502 Bad Gateway
				}

			}
		}
		// endregion

		sendMessage.Variables = map[string]string{
			dialogId: dialogId,
		}
		// region SEND GEOLOCATION
		if message.Geo != nil {
			sendMessage.Type = "text"
			sendMessage.Text = fmt.Sprintf(
				"https://www.google.com/maps/place/%f,%f",
				message.Geo.Coordinates.Latitude, message.Geo.Coordinates.Longitude,
			)
			err = c.Gateway.Read(notice.Context(), &sendUpdate)
			if err != nil {

				code := http.StatusInternalServerError
				http.Error(reply, "Failed to forward .Update recvEvent", code)
				return // 502 Bad Gateway
			}
		}
		// endregion
		if message.Text != "" {
			sendMessage.Type = "text"
			sendMessage.Text = message.Text
			err = c.Gateway.Read(notice.Context(), &sendUpdate)
			if err != nil {

				code := http.StatusInternalServerError
				http.Error(reply, "Failed to forward .Update recvEvent", code)
				return // 502 Bad Gateway
			}
		}

		// endregion
		c.BotApi.MessagesMarkAsRead(vk.Params{"peer_id": dialogId})
		reply.Write([]byte("ok"))
	case "confirmation":
		reply.Write([]byte(c.creds.ConfirmationCode))
	default:
		code := http.StatusNotImplemented
		reply.WriteHeader(code)
		return
	}
	code := http.StatusOK
	reply.WriteHeader(code)
	return
	// return // HTTP/1.1 200 OK
}

func (c *VKBot) GetUsers(userIds []int64, fields ...string) (vk.UsersGetResponse, error) {
	r, err := c.BotApi.UsersGet(vk.Params{"user_ids": strings.Trim(strings.Join(strings.Fields(fmt.Sprint(userIds)), ","), "[]"), "fields": strings.Join(fields, ",")})
	if err != nil {
		return nil, errors.BadRequest("bot.vk.get_users.error", err.Error())
	}
	return r, nil
}

func (c *VKBot) GetUser(userId int64, fields ...string) (*object.UsersUser, error) {
	r, err := c.GetUsers([]int64{userId}, fields...)
	if err != nil {
		return nil, err
	}
	return &r[0], nil
}

func (c *VKBot) getChannel(ctx context.Context, message *Message) (*bot.Channel, error) {

	chatId := strconv.FormatInt(message.PeerId, 10)
	contact := c.contacts[message.FromId]

	if contact == nil {
		user, err := c.GetUser(message.FromId)
		if err != nil {
			return nil, err
		}
		contact = &bot.Account{

			ID: 0, // LOOKUP

			Channel: "vk",
			Contact: chatId,

			FirstName: user.FirstName,
			LastName:  user.LastName,

			Username: user.Nickname,
		}
		// processed
		c.contacts[message.PeerId] = contact
	}

	return c.Gateway.GetChannel(
		ctx, chatId, contact,
	)
}

// SendPhoto as it states
func (c *VKBot) SendPhoto(photoName string, photoUrl string) (string, error) {
	var (
		uploadPhotoRequestBody bytes.Buffer
		writer                 = multipart.NewWriter(&uploadPhotoRequestBody)
		photoUploadResponse    DocUploadResponse
	)

	// region PREPARE REQUEST DATA

	// get upload url server from [VK]
	uploadUrl, err := c.BotApi.PhotosGetMessagesUploadServer(vk.Params{"group_id": c.creds.GroupId})
	if err != nil {
		return "", errors.BadRequest("bot.vk.send_photo.error", err.Error())
	}
	// get photo data from [WEBITEL]
	getPhotoResponse, err := c.BotApi.Client.Get(photoUrl)
	if err != nil {
		return "", errors.BadRequest("bot.vk.get_photo_storage.error", err.Error())
	}
	defer getPhotoResponse.Body.Close()

	// create form-data file for future request
	part, err := writer.CreateFormFile("photo", photoName)
	if err != nil {

		return "", errors.InternalServerError("bot.vk.send_photo.multipart_create.error", err.Error())
	}
	_, err = io.Copy(part, getPhotoResponse.Body)
	if err != nil {
		return "", errors.InternalServerError("bot.vk.send_photo.multipart_copy.error", err.Error())
	}

	// endregion

	// region UPLOAD REQUEST CREATION
	req, err := http.NewRequest("POST", uploadUrl.UploadURL, &uploadPhotoRequestBody)
	if err != nil {
		return "", errors.InternalServerError("bot.vk.send_photo.creating_request.error", err.Error())
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// endregion

	// region PERFORM UPLOAD
	client := &http.Client{}
	uploadPhotoResponse, err := client.Do(req)
	//	uploadPhotoResponse, err := c.BotApi.Client.Do(req)
	if err != nil {
		return "", errors.InternalServerError("bot.vk.send_photo.sending_request.error", err.Error())
	}
	defer uploadPhotoResponse.Body.Close()

	err = json.NewDecoder(uploadPhotoResponse.Body).Decode(&photoUploadResponse)
	if err != nil {
		return "", errors.BadRequest("bot.vk.unmarshal_photo.error", err.Error())
	}

	savePhotoResp, err := c.BotApi.PhotosSaveMessagesPhoto(vk.Params{"server": photoUploadResponse.Server, "hash": photoUploadResponse.Hash, "photo": photoUploadResponse.Photo})
	if err != nil {
		return "", errors.BadRequest("bot.vk.save_photo.error", err.Error())
	}

	// endregion

	// get attachment to send with message
	if len(savePhotoResp) != 0 {
		return savePhotoResp[0].ToAttachment(), nil
	} else {
		return "", errors.InternalServerError("bot.vk.send_photo.error", "vk: saved photos returned zero-length array")
	}
}

// SendDoc as it states
func (c *VKBot) SendDoc(fileName, fileUrl string, peerId int64) (string, error) {

	// region PREPARE REQUEST DATA
	uploadUrl, err := c.BotApi.DocsGetMessagesUploadServer(vk.Params{"group_id": c.creds.GroupId, "peer_id": peerId})
	if err != nil {
		return "", errors.BadRequest("bot.vk.upload_doc.error", err.Error())
	}
	r, err := c.BotApi.Client.Get(fileUrl)
	if err != nil {
		return "", errors.BadRequest("bot.vk.get_doc_storage.error", err.Error())
	}
	defer r.Body.Close()

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {

		return "", errors.InternalServerError("bot.vk.upload_doc.multipart_create.error", err.Error())
	}
	_, err = io.Copy(part, r.Body)
	if err != nil {
		return "", errors.InternalServerError("bot.vk.upload_doc.multipart_copy.error", err.Error())
	}
	writer.Close()
	// endregion

	// region UPLOAD REQUEST CREATION
	req, err := http.NewRequest("POST", uploadUrl.UploadURL, &requestBody)
	if err != nil {
		return "", errors.InternalServerError("bot.vk.send_photo.creating_request.error", err.Error())
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// endregion

	// region PERFORM UPLOAD
	uploadDocResponse, err := c.BotApi.Client.Do(req)
	if err != nil {
		return "", errors.InternalServerError("bot.vk.send_photo.sending_request.error", err.Error())
	}
	defer uploadDocResponse.Body.Close()

	var docUpload DocUploadResponse
	err = json.NewDecoder(uploadDocResponse.Body).Decode(&docUpload)
	if err != nil {
		return "", errors.BadRequest("bot.vk.unmarshal_doc.error", err.Error())
	}
	saveDocResp, err := c.BotApi.DocsSave(vk.Params{"file": docUpload.File})
	if err != nil {
		return "", errors.BadRequest("bot.vk.save_doc.error", err.Error())
	}
	// endregion

	if saveDocResp.Type != "doc" {
		return "", errors.BadRequest("bot.vk.check_doc_response.error", err.Error())

	} else {
		return saveDocResp.Doc.ToAttachment(), nil
	}

}

func unmarshalTo(src any, dst any) error {
	jsonbody, err := json.Marshal(src)
	if err != nil {
		return err
		/*return errors.BadRequest("bot.vk.parse_attachments.error", err.Error())*/
	}
	err = json.Unmarshal(jsonbody, &dst)
	if err != nil {
		return err
		/*return errors.BadRequest("bot.vk.parse_attachments.error", err.Error())*/
	}
	return nil
}

// Broadcast given `req.Message` message [to] provided `req.Peer(s)`
func (c *VKBot) BroadcastMessage(ctx context.Context, req *chat.BroadcastMessageRequest, rsp *chat.BroadcastMessageResponse) error {

	var (
		message OutgoingMessage
	)
	err := message.SetReceiver(req.GetPeer()...)
	if err != nil {
		return errors.BadRequest("bot.vk.broadcast.error", err.Error())
	}
	message.Text = req.GetMessage().GetText()
	if r := message.IsValid(); r != nil {
		return r
	}
	err = message.IsValid()
	if err != nil {
		return err
	}
	vkParams, err := message.Params()
	if err != nil {
		return err
	}
	_, err = c.BotApi.MessagesSend(*vkParams)
	if err != nil {
		return errors.BadRequest("vot.vk.broadcast.error", err.Error())
	}
	// rsp.Peers[].Erro detailed
	return nil
}
