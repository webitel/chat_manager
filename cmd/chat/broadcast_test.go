package chat

import (
	"testing"
	"time"

	"database/sql"

	"github.com/stretchr/testify/assert"
	pbmessages "github.com/webitel/chat_manager/api/proto/chat/messages"
	sqlxrepo "github.com/webitel/chat_manager/internal/repo/sqlx"
)

func TestMapChannelToAppChannel(t *testing.T) {
	createdAt := time.Now()
	updatedAt := time.Now()
	closedAt := sql.NullTime{Valid: true, Time: time.Now()}

	channel := &sqlxrepo.Channel{
		DomainID:       1,
		UserID:         123,
		Type:           "telegram",
		ID:             "456",
		ConversationID: "789",
		Variables:      map[string]string{"key": "value"},
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
		ClosedAt:       closedAt,
	}

	appChannel := mapChannelToAppChannel(channel)

	assert.NotNil(t, appChannel)
	assert.Equal(t, channel.DomainID, appChannel.DomainID)
	assert.Equal(t, channel.UserID, appChannel.User.ID)
	assert.Equal(t, channel.ID, appChannel.Chat.ID)
	assert.Equal(t, channel.ConversationID, appChannel.Chat.Invite)
	assert.EqualValues(t, channel.Variables, appChannel.Variables)
	assert.Equal(t, createdAt.Unix(), appChannel.Created)
	assert.Equal(t, updatedAt.Unix(), appChannel.Updated)
	assert.Equal(t, closedAt.Time.Unix(), appChannel.Closed)
}

func TestMapInputMessageToMessage(t *testing.T) {
	inputMessage := &pbmessages.InputMessage{
		Text: "Hello, World!",
		File: &pbmessages.InputFile{
			FileSource: &pbmessages.InputFile_Id{
				Id: "123",
			},
			Source: "media",
		},
		Keyboard: &pbmessages.InputKeyboard{
			Rows: []*pbmessages.InputButtonRow{
				{
					Buttons: []*pbmessages.InputButton{
						{Caption: "Button 1", Text: "Click Me", Type: "url", Url: "https://example.com"},
					},
				},
			},
		},
	}

	chatMessage := mapInputMessageToMessage(inputMessage)

	assert.NotNil(t, chatMessage)
	assert.Equal(t, inputMessage.GetText(), chatMessage.Text)
	assert.NotNil(t, chatMessage.File)
	assert.Equal(t, int64(123), chatMessage.File.Id)
	assert.Equal(t, "unknown/unknown; source=media", chatMessage.File.Mime)
	assert.Len(t, chatMessage.Buttons, 1)
	assert.Len(t, chatMessage.Buttons[0].Button, 1)
	assert.Equal(t, "Button 1", chatMessage.Buttons[0].Button[0].Caption)
	assert.Equal(t, "Click Me", chatMessage.Buttons[0].Button[0].Text)
	assert.Equal(t, "url", chatMessage.Buttons[0].Button[0].Type)
	assert.Equal(t, "https://example.com", chatMessage.Buttons[0].Button[0].Url)
}
