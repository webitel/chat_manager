package bot

import (
	// Register Micro Plugin(s) ...
	// _ "github.com/micro/go-plugins/broker/rabbitmq/v2"
	// _ "github.com/micro/go-plugins/registry/consul/v2"

	// Register Chat Bot Provider(s) ...
	_ "github.com/webitel/chat_manager/bot/corezoid"
	_ "github.com/webitel/chat_manager/bot/facebook"      // messenger
	_ "github.com/webitel/chat_manager/bot/telegram/gotd" // telegram-app [gotd]
	_ "github.com/webitel/chat_manager/bot/telegram/http" // telegram-bot [telegram]
	_ "github.com/webitel/chat_manager/bot/viber"
	_ "github.com/webitel/chat_manager/bot/webchat" // websocket
	_ "github.com/webitel/chat_manager/bot/whatsapp/infobip"
)
