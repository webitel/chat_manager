package cmd

import (
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/micro/micro/v3/cmd"
	"github.com/micro/micro/v3/plugin"
	"github.com/micro/micro/v3/service/auth"
	"github.com/micro/micro/v3/service/broker"
	"github.com/micro/micro/v3/service/client"
	"github.com/micro/micro/v3/service/registry"
	"github.com/urfave/cli/v2"

	authSrv "github.com/webitel/chat_manager/service/auth/noop"
	"github.com/webitel/chat_manager/service/broker/rabbitmq"
	"github.com/webitel/chat_manager/service/registry/consul"
)

var (
	onceBefore sync.Once

	Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "registry",
			Usage:   "Registry for discovery. micro, consul",
			EnvVars: []string{"MICRO_REGISTRY"},
			Value:   "consul",
		},
		&cli.StringFlag{
			Name:    "broker",
			Usage:   "Broker for pub/sub. micro, rabbitmq",
			EnvVars: []string{"MICRO_BROKER"},
			Value:   "rabbitmq",
		},
	}
)

func init() {

	os.Setenv("MICRO_PROXY", "")
	os.Setenv("MICRO_NAMESPACE", "")
	os.Setenv("MICRO_REPORT_USAGE", "false")
	os.Setenv("MICRO_PROFILE", "client")

	plugin.Register(
		plugin.NewPlugin(
			plugin.WithName("defaults"),
			plugin.WithFlag(Flags...),
			plugin.WithInit(func(ctx *cli.Context) error {
				onceBefore.Do(func() {
					// Auth: Client (disabled)
					auth.DefaultAuth = authSrv.NewAuth()
					// Client: Timeout/Selector
					client.DefaultClient.Init(
						client.RequestTimeout(
							time.Second * 30, // for media uploads
						),
						// client.Selector(
						// 	NewSelector("127.0.0.1", nil),
						// ),
					)
					// Broker: RabbitMQ
					brokerType := ctx.String("broker")
					switch strings.ToLower(brokerType) {
					case "rabbitmq":
						broker.DefaultBroker = rabbitmq.NewBroker(
							rabbitmq.ExchangeName("chat"),
							rabbitmq.DurableExchange(),
						)
					default: // "micro"
					}
					// Registry: Consul
					registryType := ctx.String("registry")
					switch strings.ToLower(registryType) {
					case "consul":
						registry.DefaultRegistry = consul.NewRegistry()
					default: // "micro"
					}
				})
				return nil
			}),
		),
	)

	app := cmd.DefaultCmd.App()
	app.Flags = append(app.Flags, Flags...)

	sort.Slice(app.Flags, func(i, j int) bool {
		return app.Flags[i].Names()[0] < app.Flags[j].Names()[0]
	})
}
