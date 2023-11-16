package bot

import (
	"net"
	"net/url"
	"os"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/webitel/chat_manager/auth"
	"github.com/webitel/chat_manager/bot"
	sqlxrepo "github.com/webitel/chat_manager/internal/repo/sqlx"
	"github.com/webitel/chat_manager/internal/wrapper"
	"github.com/webitel/chat_manager/log"
	"github.com/webitel/chat_manager/store/postgres"

	micro "github.com/micro/micro/v3/service"
	"github.com/urfave/cli/v2"

	pb "github.com/webitel/chat_manager/api/proto/bot"
	pbchat "github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/cmd"
)

const (
	// Change to REname
	name  = "webitel.chat.bot" // "chat.bot"
	usage = "Run a chat gateways service"
)

var (
	agent   pbchat.ChatService
	logger  zerolog.Logger
	service *micro.Service
	// bot     ChatServer // V0
	srv *bot.Service // *Service   // V1
	// Command Flags
	Flags = []cli.Flag{
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
			Name:  "web_root",
			Usage: "Base folder where the website additional assets are located.",
			Value: "/var/lib/webitel/public-html",
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
	}
)

func Run(ctx *cli.Context) error {

	if ctx.Bool("help") {
		cli.ShowSubcommandHelp(ctx)
		// os.Exit(1)
		return nil
	}

	service = micro.New(
		micro.Name(name),             // ("chat.bot"),
		micro.Version(cmd.Version()), // ("latest"),
		micro.WrapCall(log.CallWrapper(&logger)),
		micro.WrapHandler(log.HandlerWrapper(&logger)),
		micro.BeforeStart(func() error { return srv.Start() }),
		micro.AfterStop(func() error { return srv.Close() }),
	)

	logsLvl := ctx.String("log_level")
	baseURL := ctx.String("site_url")
	webRoot := ctx.String("web_root")
	srvAddr := ctx.String("address")
	// LOGs
	colorize := true
	stdlog, err := log.Console(logsLvl, colorize) // NewLogger(cfg.LogLevel)
	if err != nil {
		return err
	}

	log.Default = *(stdlog)
	logger = log.Default // *(stdlog)

	// CHECK: valid [host]:port address specified
	if _, _, err := net.SplitHostPort(srvAddr); err != nil {
		return errors.Wrap(err, "Invalid address")
	}
	// CHECK: valid URL specified
	if _, err := url.Parse(baseURL); err != nil {
		return errors.Wrap(err, "Invalid URL")
	}
	// CHECK: web_root folder exists
	rootDir, err := os.Stat(webRoot)
	if os.IsNotExist(err) {
		return errors.Wrap(err, "--web_root")
	}
	if !rootDir.IsDir() {
		return errors.New("--web_root: directory required")
	}

	sender := service.Client()                                                          // Micro-From-Service: webitel.chat.bot
	sender = wrapper.FromService(service.Name(), service.Server().Options().Id, sender) // Micro-From-Id: server.DefaultId
	agent = pbchat.NewChatService("webitel.chat.server", sender)

	// cfg.CertPath = c.String("cert_path")
	// cfg.KeyPath = c.String("key_path")
	// cfg.ConversationTimeout = c.Uint64("conversation_timeout")

	// Open persistent [D]ata[S]ource[N]ame ...
	dbo, err := postgres.OpenDB(ctx.String("db-dsn"))
	if err != nil {
		return errors.Wrap(err, "Invalid DSN String")
	}

	// ping, cancel := context.WithTimeout(context.TODO(), time.Second*5)
	// err = dbo.DB.PingContext(ping)
	// cancel()

	// if err != nil {
	// 	// logger.Fatal().Msg()
	// 	return errors.Wrap(err, "Connect DSN Failed")
	// }

	// configure
	store := sqlxrepo.NewBotStore(&logger, dbo.DB)
	srv = bot.NewService(store, &logger, agent)
	srv.WebRoot = webRoot // Static assets base folder

	// AUTH: go.webitel.app
	srv.Auth = auth.NewClient(
		auth.ClientService(service),
		auth.ClientCache(auth.NewLru(4096)),
	)

	for _, regErr := range []error{
		pb.RegisterBotsHandler(service.Server(), srv),
	} {
		if regErr != nil {
			logger.Fatal().
				Str("app", "failed to register service").
				Msg(regErr.Error())
			return regErr
		}
	}

	srv.URL = baseURL
	srv.Addr = srvAddr

	// r.PathPrefix("/").Methods("GET", "POST").Handler(srv)

	// Run the server
	if err := service.Run(); err != nil {
		logger.Fatal().
			Str("app", "failed to run service").
			Msg(err.Error())
	}

	return nil
}

func init() {
	app := &cli.Command{
		Name:   "bot",
		Usage:  usage,
		Flags:  Flags,
		Action: Run,
	}
	cmd.Register(app)
}
