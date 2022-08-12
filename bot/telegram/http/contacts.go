package telegram

import "github.com/webitel/chat_manager/bot"

type contacts struct {
	telegramId map[int64]*bot.Account
}

func (c contacts) resolveUserId(id int64) *bot.Account {
	return c.telegramId[id]
}
