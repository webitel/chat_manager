package main

import (
	"context"
	"net"
	"net/url"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/webitel/chat_manager/auth"
	"github.com/webitel/chat_manager/bot"
	sqlxrepo "github.com/webitel/chat_manager/internal/repo/sqlx"
	"github.com/webitel/chat_manager/internal/wrapper"
	"github.com/webitel/chat_manager/log"
	"github.com/webitel/chat_manager/store/postgres"

	"github.com/micro/cli/v2"
	"github.com/micro/go-micro/v2"

	pb "github.com/webitel/chat_manager/api/proto/bot"
	pbchat "github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/cmd"
)

type Config struct {
	LogLevel string
	SiteURL  string // Public HTTP server site URL, e.g.: "https://example.com/chat"
	Address  string // Bind HTTP server address, e.g.: "localhost:10031"
	// CertPath string
	// KeyPath  string
	DBSource string // Database DSN, e.g.: postgres://user:pass@host:5432/webitel?sslmode=disable&connect_timeout=5
}

var (
	agent  pbchat.ChatService
	logger  zerolog.Logger
	cfg     *Config
	service micro.Service
	// bot     ChatServer // V0
	srv     *bot.Service // *Service   // V1
)

// func init() {
// 	// plugins
// 	cmd.DefaultRegistries["consul"] = consul.NewRegistry
// }

func main() {

	cfg = &Config{}

	service = micro.NewService(
		micro.Name("webitel.chat.bot"), // ("chat.bot"),
		micro.Version(cmd.Version()), // ("latest"),
		micro.Flags(
			&cli.BoolFlag{
				Name:    "version",
				Aliases: []string{"ver"},
				Usage:   "Show build version and exit",
			},
			&cli.StringFlag{
				Name:    "log_level",
				EnvVars: []string{"LOG_LEVEL"},
				Value:   "debug",
				Usage:   "Log Level",
			},
			&cli.StringFlag{
				Name:    "site_url",
				EnvVars: []string{"WEBITEL_BOT_PROXY"},
				Usage:   "Public HTTP site URL used when registering webhooks with BOT providers.",
				// Value: "https://example.com/chat", // TODO: use http[s]://${address} if blank
			},
			&cli.StringFlag{
				Name:    "address",
				EnvVars: []string{"WEBITEL_BOT_ADDRESS"},
				Usage:   "Bind address for the HTTP server.",
				Value:   "127.0.0.1:10030", // default
			},
			&cli.StringFlag{
				Name:    "db-dsn",
				EnvVars: []string{"WEBITEL_DBO_ADDRESS"},
				Usage:   "Persistent database driver name and a driver-specific data source name.",
			},
			// // TODO: remove below !
			// &cli.StringFlag{
			// 	Name:    "cert_path",
			// 	EnvVars: []string{"CERT_PATH"},
			// 	Usage:   "SSl certificate",
			// },
			// &cli.StringFlag{
			// 	Name:    "key_path",
			// 	EnvVars: []string{"KEY_PATH"},
			// 	Usage:   "SSl key",
			// },
		),
		micro.WrapHandler(log.HandlerWrapper(&logger)),
		micro.WrapCall(log.CallWrapper(&logger)),
	)

	service.Init(
		micro.Action(func(c *cli.Context) error {

			if c.Bool("version") {
				return cmd.ShowVersion(c)
			}

			cfg.LogLevel = c.String("log_level")
			cfg.SiteURL = c.String("site_url")
			cfg.Address = c.String("address")
			// cfg.CertPath = c.String("cert_path")
			// cfg.KeyPath = c.String("key_path")
			// cfg.ConversationTimeout = c.Uint64("conversation_timeout")
			cfg.DBSource = c.String("db-dsn")

			// CHECK: valid [host]:port address specified
			if _, _, err := net.SplitHostPort(cfg.Address); err != nil {
				return errors.Wrap(err, "Invalid address")
			}
			// CHECK: valid URL specified
			if _, err := url.Parse(cfg.SiteURL); err != nil {
				return errors.Wrap(err, "Invalid URL")
			}

			stdlog, err := log.Console(cfg.LogLevel, true) // NewLogger(cfg.LogLevel)
			if err != nil {
				return err
			}
			
			log.Default = *(stdlog)
			logger = log.Default // *(stdlog)

			sender := service.Client() // Micro-From-Service: webitel.chat.bot
			sender = wrapper.FromServiceId(service.Server().Options().Id, sender) // Micro-From-Id: server.DefaultId
			agent = pbchat.NewChatService("webitel.chat.server", sender)
			
			return configure()
		}),
		micro.BeforeStart(
			func() error {
				return srv.Start()
			},
		),
		micro.AfterStop(
			func() error {
				return srv.Close()
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
	// Parse [D]ata[S]ource[N]ame ...
	dbo, err := postgres.OpenDB(cfg.DBSource)

	if err != nil {
		return errors.Wrap(err, "Invalid DSN String")
	}

	ctx, cancel := context.WithTimeout(context.TODO(), time.Second * 5)
	err = dbo.DB.PingContext(ctx)
	cancel()

	if err != nil {
		// logger.Fatal().Msg()
		return errors.Wrap(err, "Connect DSN Failed")
	}

	store := sqlxrepo.NewBotStore(&logger, dbo.DB)
	
	// r := mux.NewRouter()
	
	// if cfg.LogLevel == "trace" {
	// 	r.Use(dumpMiddleware)
	// }

	// srv = NewService(&logger, agent)
	srv = bot.NewService(store, &logger, agent)

	// AUTH: go.webitel.app
	srv.Auth = auth.NewClient(
		auth.ClientService(service),
		auth.ClientCache(auth.NewLru(4096)),
	)
	
	// err := pb.RegisterBotServiceHandler(service.Server(), srv)
	err = pb.RegisterBotsHandler(service.Server(), srv)
	
	if err != nil {
		logger.Fatal().
			Str("app", "failed to register service").
			Msg(err.Error())
		return err
	}

	srv.URL = cfg.SiteURL
	srv.Addr = cfg.Address

	// r.PathPrefix("/").Methods("GET", "POST").Handler(srv)

	return nil
}