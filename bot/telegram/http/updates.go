package telegram

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	telegram "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/webitel/chat_manager/bot"
)

func (c *TelegramBot) getChannel(ctx context.Context, chat telegram.Chat) (*bot.Channel, error) {

	contact := c.contacts[chat.ID]
	chatId := strconv.FormatInt(chat.ID, 10)

	if contact == nil {
		contact = &bot.Account{

			ID: 0, // LOOKUP

			Channel: "telegram",
			Contact: chatId,

			FirstName: chat.FirstName,
			LastName:  chat.LastName,

			Username: chat.UserName,
		}
		// processed
		c.contacts[chat.ID] = contact
	}

	return c.Gateway.GetChannel(
		ctx, chatId, contact,
	)
}

func (c *TelegramBot) onMyChatMember(ctx context.Context, e *telegram.ChatMemberUpdated) {
	switch e.NewChatMember.Status {
	// a chat member that has no additional privileges or restrictions
	// https://core.telegram.org/bots/api#chatmembermember
	case "member":
		// MUST: kicked => member; Member just RESTARTed our bot. Welcome !
		user := e.From
		c.Gateway.Log.Info("TELEGRAM: RESTART",
			slog.Int64("id", e.Chat.ID),
			slog.String("username", user.UserName),
			slog.String("dialog", strings.TrimSpace(
				strings.Join([]string{user.FirstName, user.LastName}, " "),
			)),
			slog.String("notice", "Bot was restarted by the user"),
		)
	// Our bot, as a member, was banned in the chat
	// and can't return to the chat or view chat messages.
	// https://core.telegram.org/bots/api#chatmemberbanned
	case "kicked":

		user := e.From
		c.Gateway.Log.Warn("TELEGRAM: STOP & BLOCK",
			slog.Int64("id", e.Chat.ID),
			slog.String("username", user.UserName),
			slog.String("dialog", strings.TrimSpace(
				strings.Join([]string{user.FirstName, user.LastName}, " "),
			)),
			slog.String("error", "Bot was blocked by the user"),
		)
		// Force close active conversation dialog
		dialog, err := c.getChannel(ctx, e.Chat)
		if err == nil && !dialog.IsNew() {
			_ = dialog.Close()
		}
	}
}

// func (c *TelegramBot) onNewMessage(ctx context.Context, e *telegram.Message) {
// 	// TODO: Optimize c.WebHook() handler
// }
