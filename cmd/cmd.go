package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/micro/micro/v3/cmd"
	"github.com/urfave/cli/v2"
)

var DefaultCmd = cmd.DefaultCmd

func init() {

	DefaultCmd.Init(
		cmd.Name("messages"),
		cmd.Description("Webitel Messages Micro Service(s)\n\n	 Use `messages [command] --help` to see command specific help."),
		cmd.Version(Version()),
		cmd.Before(func(ctx *cli.Context) (err error) {
			_ = ctx.Set("report_usage", "false")
			_ = ctx.Set("proxy_address", "")
			_ = ctx.Set("namespace", "")
			return nil
		}),
	)
}

// Register CLI commands
func Register(cmds ...*cli.Command) {
	app := DefaultCmd.App()
	app.Commands = append(app.Commands, cmds...)

	// sort the commands so they're listed in order on the cli
	// todo: move this to micro/cli so it's only run when the
	// commands are printed during "help"
	sort.Slice(app.Commands, func(i, j int) bool {
		return app.Commands[i].Name < app.Commands[j].Name
	})
}

// Run the default command
func Run() {
	if err := DefaultCmd.Run(); err != nil {
		fmt.Println(err) // formatErr(err))
		os.Exit(1)
	}
}
