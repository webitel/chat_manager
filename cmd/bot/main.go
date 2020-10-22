package main

import (
	"os"

	pb "github.com/webitel/protos/pkg/bot"
	pbchat "github.com/webitel/protos/pkg/chat"

	"github.com/gorilla/mux"
	"github.com/micro/cli/v2"
	"github.com/micro/go-micro/v2"
	"github.com/micro/go-micro/v2/config/cmd"
	"github.com/micro/go-plugins/registry/consul/v2"
	"github.com/rs/zerolog"
)

type Config struct {
	LogLevel string
	Webhook  string
	CertPath string
	KeyPath  string
	AppPort  int
}

var (
	client  pbchat.ChatService
	logger  *zerolog.Logger
	cfg     *Config
	service micro.Service
	bot     ChatServer
)

func init() {
	// plugins
	cmd.DefaultRegistries["consul"] = consul.NewRegistry
}

func main() {
	cfg = &Config{}

	service = micro.NewService(
		micro.Name("webitel.chat.bot"),
		micro.Version("latest"),
		micro.Flags(
			&cli.StringFlag{
				Name:    "log_level",
				EnvVars: []string{"LOG_LEVEL"},
				Value:   "debug",
				Usage:   "Log Level",
			},
			&cli.StringFlag{
				Name:    "webhook_address",
				EnvVars: []string{"WEBHOOK_ADDRESS"},
				Usage:   "Webhook address",
			},
			&cli.IntFlag{
				Name:    "app_port",
				EnvVars: []string{"APP_PORT"},
				Usage:   "Local webhook port",
			},
			&cli.StringFlag{
				Name:    "cert_path",
				EnvVars: []string{"CERT_PATH"},
				Usage:   "SSl certificate",
			},
			&cli.StringFlag{
				Name:    "key_path",
				EnvVars: []string{"KEY_PATH"},
				Usage:   "SSl key",
			},
		),
	)

	service.Init(
		micro.Action(func(c *cli.Context) error {
			cfg.LogLevel = c.String("log_level")
			cfg.Webhook = c.String("webhook_address")
			cfg.CertPath = c.String("cert_path")
			cfg.KeyPath = c.String("key_path")
			cfg.AppPort = c.Int("app_port")
			// cfg.ConversationTimeout = c.Uint64("conversation_timeout")

			client = pbchat.NewChatService("webitel.chat.server", service.Client())
			var err error
			logger, err = NewLogger(cfg.LogLevel)
			if err != nil {
				return err
			}
			return configure()
		}),
		micro.AfterStart(
			func() error {
				return bot.StartWebhookServer()
			},
		),
		micro.AfterStop(
			func() error {
				return bot.StopWebhookServer()
			},
		),
	)

	if err := service.Run(); err != nil {
		logger.Fatal().
			Str("app", "failed to run service").
			Msg(err.Error())
	}
}

func configure() error {
	r := mux.NewRouter()

	bot = NewBotService(
		logger,
		client,
		r,
	)

	if err := pb.RegisterBotServiceHandler(service.Server(), bot); err != nil {
		logger.Fatal().
			Str("app", "failed to register service").
			Msg(err.Error())
		return err
	}
	return nil
}

func NewLogger(logLevel string) (*zerolog.Logger, error) {
	lvl, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		return nil, err
	}

	l := zerolog.New(os.Stdout).With().Timestamp().Logger()
	l = l.Level(lvl)

	return &l, nil
}
