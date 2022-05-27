package main

import (
	"github.com/webitel/chat_manager/cmd"

	// load packages so they can register commands
	// _ "github.com/micro/micro/v3/client/cli"
	// _ "github.com/micro/micro/v3/cmd/server"
	// _ "github.com/micro/micro/v3/cmd/service"
	// _ "github.com/micro/micro/v3/cmd/usage"

	_ "github.com/webitel/chat_manager/cmd/bot"
	_ "github.com/webitel/chat_manager/cmd/chat"
)

func main() {
	cmd.Run()
}
