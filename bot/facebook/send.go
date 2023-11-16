package facebook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"path"

	"github.com/micro/micro/v3/service/errors"
	"github.com/webitel/chat_manager/bot/facebook/messenger"
)

func (c *Client) Send(from *Page, send *messenger.SendRequest) (mid string, err error) {

	body := bytes.NewBuffer(nil)
	err = json.NewEncoder(body).Encode(send)

	if err != nil {
		// Failed to encode SendAPI request body
		return "", err
	}

	// POST /me/messages?access_token=PAGE_ACCESS_TOKEN"
	// HOST graph.facebook.com
	//
	// NOTE: A page access token with pages_messaging permission is required to interact with this endpoint.

	req, err := http.NewRequest(http.MethodPost,
		"https://graph.facebook.com"+path.Join("/", c.Version, "me/messages")+
			"?"+c.requestForm(nil, from.AccessToken).Encode(),
		body,
	)

	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	rsp, err := c.Client.Do(req)
	if err != nil {
		// Failed to send GraphAPI HTTP Request
		return "", err
	}
	defer rsp.Body.Close()

	var res messenger.SendResponse
	err = json.NewDecoder(rsp.Body).Decode(&res)

	if err != nil {
		// Failed to decode API Response
		return "", err
	}

	if res.Error != nil {
		// SendAPI Call Error !
		return res.MessageID, res.Error
	}

	return res.MessageID, nil
}

// A page access token with `pages_messaging` permission is required to interact with this endpoint.
// https://developers.facebook.com/docs/messenger-platform/reference/send-api
func (c *Client) SendText(senderPageId, recepientUserId, messageText string) (mid string, err error) {

	senderPage := c.pages.getPage(senderPageId)
	if senderPage == nil || !senderPage.IsAuthorized() {
		return "", errors.NotFound(
			"bot.messenger.page.not_found",
			"messenger: page=%s not found",
			senderPageId,
		)
	}

	send := messenger.SendRequest{
		// https://developers.facebook.com/docs/messenger-platform/send-messages/#messaging_types
		Type: "RESPONSE",
		Recipient: &messenger.SendRecipient{
			ID: recepientUserId,
		},
		Message: &messenger.SendMessage{
			Text: messageText,
		},
		// Notify: "REGULAR",
		// Tag: "",
	}

	return c.Send(senderPage, &send)
}

// A page access token with `pages_messaging` permission is required to interact with this endpoint.
// https://developers.facebook.com/docs/messenger-platform/reference/send-api
func (c *Client) SendInstagramText(igpage *Page, recepientUserId, messageText string) (mid string, err error) {

	send := messenger.SendRequest{
		// https://developers.facebook.com/docs/messenger-platform/send-messages/#messaging_types
		Type: "RESPONSE",
		Recipient: &messenger.SendRecipient{
			ID: recepientUserId,
		},
		Message: &messenger.SendMessage{
			Text: messageText,
		},
		// Notify: "REGULAR",
		// Tag: "",
	}

	return c.Send(igpage, &send)
}
