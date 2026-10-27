package bot

import (
	"maps"
	"strings"

	pb "github.com/webitel/chat_manager/api/proto/bot"
	"github.com/webitel/chat_manager/app"
)

// metadata public view attributes map
// general for all kind of provider(s)
var metadataMask = struct{
	suppress []string // set of attributes with sensitive (secret) data to be suppressed from view
	internal []string // set of attributes, holding provider's internal state, hidden from view
} {
	suppress: []string{
		// viber | telegram
		"token",
		// custom
		"secret",
		// infobip_whatsapp
		"api_key",
		// telegram (gotd)
		"api_hash",
		// messenger
		"client_secret",
		// messenger:whatsapp
		"whatsapp_token",
	},
	internal: []string{
		// telegram (gotd)
		".auth",
		".gotd",
		// messenger
		"fb", // facebook: pages
		"ig", // instagram: pages
		"wa", // whatsapp: numbers
	},
}

// marshal given source [data] for public [view]
// - hide internal state attributes
// - suppress secret attributes value
func metadataView(data map[string]string) (view map[string]string) {
	if len(data) == 0 {
		return nil
	}
	view = maps.Clone(data) // copy
	for _, att := range metadataMask.internal {
		delete(view, att) // remove
	}
	for _, att := range metadataMask.suppress {
		if vs, ok := view[att]; ok {
			vs = secretView.Suppress(vs)
			if vs != "" {
				view[att] = vs
			} else {
				delete(view, att)
			}
		}
	}
	return view
}

// merge [dst] metadata changes with known [src] state
// - populate [internal] attributes from [src] back to [dst]
// - populate [original] secret(s) from [src] instead of [dst] suppressed
func mergeMetadata(dst, src map[string]string) {
	if len(src) == 0 {
		return // dst
	}
	if dst == nil {
		dst = make(map[string]string, len(src))
	}
	// provide internal state from source
	for _, att := range metadataMask.internal {
		v1, ok := src[att]
		if ok && v1 != "" {
			dst[att] = v1
		}
	}
	// passthru supressed secret(s) from GET request ?
	for _, att := range metadataMask.suppress {
		v2, ok := dst[att]
		if ok && secretView.Suppressed(v2) {
			dst[att] = src[att]
		}
	}
	// cleanup
	for att, v2 := range dst {
		if v2 == "" {
			delete(dst, att)
		}
	}
}

// prepares public gateway [view] from given source [data] for set of [fields] only
// NOTE: [fields] filter not implemented yet ; all populated [data] fields are returned
func marshalTextGateView(view, data *Bot, fields []string) {
	// copy populated [fields] from source [data] to result [view]
	app.MergeProto(view, data, fields...)
	// prepare public metadata view
	view.Metadata = metadataView(view.Metadata)
}

// prepares public gateway [list] from given source [data] with [opts] been requested
func marshalTextGateList(list *pb.SearchBotResponse, data []*Bot, opts *app.SearchOptions) {
	// result [page] number ; as requested
	list.Page = max(1, int32(opts.GetPage()))
	size := len(data)
	if size == 0 {
		// no data
		return
	}
	limit := int(opts.GetSize())
	// crop dataset up to requested size limit
	list.Next = ( 0 < limit && limit < size )
	if list.Next { size = limit } // view
	
	var (
		// tidy memory page
		page = make([]pb.Bot, size)
		view = make([]*pb.Bot, size)
	)
	
	for e, item := range data[:size] {
		marshalTextGateView(&page[e], item, opts.Fields)
		view[e] = &page[e]
	}
	// populate result dataset
	list.Items = view
}

var secretView = SecretViewOptions{
	Mask: strings.Repeat("*", 10),
	View: -4, // show last 4 characters
}

type SecretViewOptions struct {
	Mask string
	View int
}

// Suppress given secret [s] from public view
func (x SecretViewOptions) Suppress(s string) (vs string) {
	c := len(s)
	if c == 0 {
		// no value
		return ""
	}
	// default: mask
	vs = x.Mask
	// disclose chars ?
	if x.View == 0 {
		// NO ; all are hidden ..
		return vs
	}
	view := x.View
	last := (view < 0)
	if last {
		view *= -1
	}
	view = min(view, c)
	if last {
		vs += s[c-view:]
	} else {
		vs = s[:view] + vs
	}
	return vs
}

// Suppressed reports whether given string [vs] looks like result of Suppress() method
func (x SecretViewOptions) Suppressed(vs string) (is bool) {
	view := x.View
	last := (view < 0)
	if last {
		view *= -1
		vs, is = strings.CutPrefix(vs, x.Mask)
		return is && len(vs) <= view
	}
	if view > 0 {
		vs, is = strings.CutSuffix(vs, x.Mask)
		return is && len(vs) <= view
	}
	return len(x.Mask) > 0 && vs == x.Mask
}