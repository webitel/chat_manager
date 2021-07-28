package main

import (
	// Register Micro Plugin(s) ...
	_ "github.com/micro/go-plugins/broker/rabbitmq/v2"
	_ "github.com/micro/go-plugins/registry/consul/v2"

	// Register Chat Bot Provider(s) ...
	_ "github.com/webitel/chat_manager/bot/corezoid"
	_ "github.com/webitel/chat_manager/bot/facebook"
	_ "github.com/webitel/chat_manager/bot/telegram"

	_ "github.com/webitel/chat_manager/bot/infobip_whatsapp" // infobip
	_ "github.com/webitel/chat_manager/bot/viber"
	_ "github.com/webitel/chat_manager/bot/webchat" // websocket
)