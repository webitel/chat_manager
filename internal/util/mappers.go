package util

import (
	"mime"
	"strconv"

	pbchat "github.com/webitel/chat_manager/api/proto/chat"
	pbmessages "github.com/webitel/chat_manager/api/proto/chat/messages"
	"github.com/webitel/chat_manager/app"
	sqlxrepo "github.com/webitel/chat_manager/internal/repo/sqlx"
)

// MapChannelToAppChannel transform sqlxrepo.Channel struct to app.Channel struct
func MapChannelToAppChannel(channel *sqlxrepo.Channel) *app.Channel {
	firstName, lastName := ParseFullName(channel.FullName())

	appChannel := app.Channel{
		DomainID: channel.DomainID,
		User: &app.User{
			ID:        channel.UserID,
			Channel:   channel.Type,
			FirstName: firstName,
			LastName:  lastName,
		},
		Chat: &app.Chat{
			ID:        channel.ID,
			Channel:   channel.Type,
			FirstName: firstName,
			LastName:  lastName,
			Invite:    channel.ConversationID,
		},
		Variables: channel.Variables,
		Created:   channel.CreatedAt.Unix(),
		Updated:   channel.UpdatedAt.Unix(),
		Closed:    0,
	}

	if channel.ClosedAt.Valid {
		appChannel.Closed = channel.ClosedAt.Time.Unix()
	}

	return &appChannel
}

// MapInputMessageToMessage transform pbmessages.InputMessage struct to pbchat.Message struct
func MapInputMessageToMessage(inputMessage *pbmessages.InputMessage) *pbchat.Message {

	// NOTE: Get file and keyboard from input message
	file := inputMessage.GetFile()
	keyboard := inputMessage.GetKeyboard()

	// NOTE: Set chat message text
	chatMessage := &pbchat.Message{
		Text: inputMessage.GetText(),
	}

	// NOTE: Set chat message file
	if file != nil {
		chatFile := &pbchat.File{}
		if file.GetId() != "" {
			parsedFileId, err := strconv.ParseInt(file.GetId(), 10, 64)
			if err == nil && parsedFileId > 0 {
				chatFile.Id = parsedFileId
			}
			chatFile.Mime = mime.FormatMediaType("unknown/unknown", map[string]string{
				"source": file.GetSource(),
			})
		} else if file.GetUrl() != "" {
			chatFile.Url = file.GetUrl()
		}
		chatMessage.File = chatFile
	}

	// NOTE: Set chat keyboard DTO
	if keyboard != nil {
		for _, row := range keyboard.GetRows() {
			chatButtons := &pbchat.Buttons{}
			for _, button := range row.GetButtons() {
				chatButtons.Button = append(chatButtons.Button, &pbchat.Button{
					Caption: button.Caption,
					Text:    button.Text,
					Type:    button.Type,
					Url:     button.Url,
					Code:    button.Code,
				})
			}

			chatMessage.Buttons = append(chatMessage.Buttons, chatButtons)
		}
	}

	return chatMessage
}
