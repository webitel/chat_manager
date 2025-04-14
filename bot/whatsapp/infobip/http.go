package infobip

import (
	"net/http"

	"github.com/webitel/chat_manager/bot"
)

func (c *App) httpMediaClient() (client *http.Client) {
	client = c.media
	if client != nil {
		return // client
	}
	configure := *(http.DefaultClient) // shallowcopy
	transport := configure.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	switch tr := transport.(type) {
	case *bot.TransportDump:
		if tr.WithBody {
			tr = &bot.TransportDump{
				Transport: tr.Transport,
				WithBody:  false,
			}
			transport = tr
		}
	default:
		transport = &bot.TransportDump{
			Transport: transport,
			WithBody:  false,
		}
	}
	configure.Transport = transport
	client = &configure
	c.media = client
	return // client
}
