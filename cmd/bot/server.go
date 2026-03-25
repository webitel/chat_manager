package bot

import (
	"log/slog"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	micro "github.com/micro/micro/v3/service"
	"github.com/micro/micro/v3/service/broker"
	"github.com/micro/micro/v3/service/client"
	"github.com/micro/micro/v3/service/server"
	microgrpcsrv "github.com/micro/micro/v3/service/server/grpc"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"google.golang.org/grpc"

	otelsdk "github.com/webitel/webitel-go-kit/otel/sdk"

	pb "github.com/webitel/chat_manager/api/proto/bot"
	pbchat "github.com/webitel/chat_manager/api/proto/chat"
	audProto "github.com/webitel/chat_manager/api/proto/logger"
	pbstorage "github.com/webitel/chat_manager/api/proto/storage"
	"github.com/webitel/chat_manager/auth"
	"github.com/webitel/chat_manager/bot"
	"github.com/webitel/chat_manager/cmd"
	sqlxrepo "github.com/webitel/chat_manager/internal/repo/sqlx"
	"github.com/webitel/chat_manager/internal/wrapper"
	"github.com/webitel/chat_manager/log"
	aud "github.com/webitel/chat_manager/logger"
	"github.com/webitel/chat_manager/otel"
	"github.com/webitel/chat_manager/store/postgres"
)

const (
	// Change to REname
	name  = "webitel.chat.bot" // "chat.bot"
	usage = "Run a chat gateways service"
)

var (
	srv *bot.Service

	agent   pbchat.ChatService
	service *micro.Service

	Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "log_level",
			EnvVars: []string{"WBTL_LOG_LEVEL"},
			Usage:   "Log Level",
		},
		&cli.StringFlag{
			Name:    "site_url",
			EnvVars: []string{"WEBITEL_BOT_PROXY"},
			Usage:   "Public HTTP site URL used when registering webhooks with BOT providers.",
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
			Value:   "127.0.0.1:10030",
		},
		&cli.StringFlag{
			Name:    "db-dsn",
			EnvVars: []string{"WEBITEL_DBO_ADDRESS"},
			Usage:   "Persistent database driver name and a driver-specific data source name.",
		},
		&cli.IntFlag{
			Name:    "db-max-open-conns",
			EnvVars: []string{"WEBITEL_DBO_MAX_OPEN_CONNS"},
			Usage:   "Maximum number of open connections to the database.",
			Value:   25,
		},
		&cli.IntFlag{
			Name:    "db-max-idle-conns",
			EnvVars: []string{"WEBITEL_DBO_MAX_IDLE_CONNS"},
			Usage:   "Maximum number of idle connections in the pool.",
			Value:   25,
		},
		&cli.DurationFlag{
			Name:    "db-conn-max-idle-time",
			EnvVars: []string{"WEBITEL_DBO_CONN_MAX_IDLE_TIME"},
			Usage:   "Maximum amount of time a connection may be idle before being closed.",
			Value:   1 * time.Minute,
		},
		&cli.DurationFlag{
			Name:    "db-conn-max-lifetime",
			EnvVars: []string{"WEBITEL_DBO_CONN_MAX_LIFETIME"},
			Usage:   "Maximum amount of time a connection may be reused.",
			Value:   5 * time.Minute,
		},
	}
)

func Run(ctx *cli.Context) error {
	if ctx.Bool("help") {
		cli.ShowSubcommandHelp(ctx)
		// os.Exit(1)
		return nil
	}

	version := cmd.GitTag
	if version == "" {
		version = "dev"
	}
	resourceAttrs := []attribute.KeyValue{
		semconv.ServiceName(name),
		semconv.ServiceVersion(version),
		semconv.ServiceInstanceID(server.DefaultId),
	}
	for key, value := range map[string]string{
		"service.version.build": cmd.BuildDate,
		"service.version.patch": cmd.GitCommit,
	} {
		if value = strings.TrimSpace(value); value != "" {
			resourceAttrs = append(resourceAttrs,
				attribute.String(key, value),
			)
		}
	}

	err := otel.Configure(ctx.Context,
		otelsdk.WithResource(resource.NewSchemaless(
			resourceAttrs...,
		)),
	)
	if err != nil {
		// fatal(fmt.Errorf("[O]pen[Tel]emetry configuration failed"))
		return err // log(FATAL) & exit(1)
	}

	defer otel.Shutdown(ctx.Context)
	stdlog := slog.Default()

	server := server.DefaultServer
	err = server.Init(microgrpcsrv.Options(
		grpc.StatsHandler(otelgrpc.NewServerHandler(
			// ...otelgrpc.Option
			otelgrpc.WithMessageEvents(
				otelgrpc.ReceivedEvents,
				otelgrpc.SentEvents,
			),
		)),
	))
	if err != nil {
		// Failed to setup micro/grpc.Server
		return err
	}

	service = micro.New(
		micro.Name(name),             // ("chat.bot"),
		micro.Version(cmd.Version()), // ("latest"),
		micro.WrapCall(log.CallWrapper(stdlog)),
		micro.WrapHandler(log.HandlerWrapper(stdlog)),
		micro.BeforeStart(func() error { return srv.Start() }),
		micro.AfterStop(func() error { return srv.Close() }),
	)

	// logsLvl := ctx.String("log_level")
	baseURL := ctx.String("site_url")
	webRoot := ctx.String("web_root")
	srvAddr := ctx.String("address")

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

	// Micro-From-Service: webitel.chat.bot
	sender := service.Client()
	// Micro-From-Id: server.DefaultId
	sender = wrapper.FromService(
		service.Name(), service.Server().Options().Id, sender,
	)
	// Trace-Id
	err = sender.Init(client.WrapCall(wrapper.OtelMicroCall))
	if err != nil {
		// Failed to setup micro/gRPC.Client(webitel.chat.server)
		return err
	}
	agent = pbchat.NewChatService("webitel.chat.server", sender)

	// cfg.CertPath = c.String("cert_path")
	// cfg.KeyPath = c.String("key_path")
	// cfg.ConversationTimeout = c.Uint64("conversation_timeout")

	// Open persistent [D]ata[S]ource[N]ame ...
	dbopts := []postgres.Option{
		postgres.WithMaxOpenConns(ctx.Int("db-max-open-conns")),
		postgres.WithMaxIdleConns(ctx.Int("db-max-idle-conns")),
		postgres.WithConnMaxIdleTime(ctx.Duration("db-conn-max-idle-time")),
		postgres.WithConnMaxLifetime(ctx.Duration("db-conn-max-lifetime")),
	}

	dbo, err := postgres.OpenDB(stdlog, ctx.String("db-dsn"), dbopts...)
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
	store := sqlxrepo.NewBotStore(stdlog, dbo.DB)
	auditor := aud.NewClient(broker.DefaultBroker, audProto.NewConfigService("logger", sender))
	fileService := pbstorage.NewFileService("storage", sender)
	srv = bot.NewService(store, stdlog, agent, auditor, fileService)
	srv.WebRoot = webRoot // Static assets base folder

	// AUTH: go.webitel.app
	srv.Auth = auth.NewClient(
		auth.ClientService(service),
		auth.ClientCache(auth.NewLru(4096)),
	)

	for _, regErr := range []error{
		pb.RegisterBotsHandler(service.Server(), srv),
		// register more micro/gRPC service(s) here ...
	} {
		if regErr != nil {
			log.FataLog(nil, // nil,
				"failed to register service",
				slog.Any("error", regErr),
			)
			return regErr
		}
	}

	srv.URL = baseURL
	srv.Addr = srvAddr

	// r.PathPrefix("/").Methods("GET", "POST").Handler(srv)

	// Run the server
	if err := service.Run(); err != nil {
		log.FataLog(nil, // nil,
			"failed to run service",
			slog.Any("error", err),
		)
		return err
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
