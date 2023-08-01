package chat

import (
	"net/http"
	"os"

	micro "github.com/micro/micro/v3/service"
	"github.com/micro/micro/v3/service/broker"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
	pbauth "github.com/webitel/chat_manager/api/proto/auth"
	pbbot "github.com/webitel/chat_manager/api/proto/bot"
	pb "github.com/webitel/chat_manager/api/proto/chat"
	pb2 "github.com/webitel/chat_manager/api/proto/chat/messages"
	pbstorage "github.com/webitel/chat_manager/api/proto/storage"
	pbmanager "github.com/webitel/chat_manager/api/proto/workflow"

	"github.com/webitel/chat_manager/cmd"
	"github.com/webitel/chat_manager/log"

	authN "github.com/webitel/chat_manager/auth"
	"github.com/webitel/chat_manager/internal/auth"
	event "github.com/webitel/chat_manager/internal/event_router"
	"github.com/webitel/chat_manager/internal/flow"
	pg "github.com/webitel/chat_manager/internal/repo/sqlx"
	"github.com/webitel/chat_manager/internal/wrapper"
)

const (
	name  = "webitel.chat.server" // "chat.srv"
	usage = "Run a chat messages service"
)

var (
	service *micro.Service
	logger  = zerolog.New(os.Stdout)
	//redisStore store.Store
	// rabbitBroker broker.Broker
	//redisTable    string
	flowClient    pbmanager.FlowChatServerService
	botClient     pbbot.BotsService // pbbot.BotService
	authClient    pbauth.AuthService
	storageClient pbstorage.FileService
	timeout       uint64
	// Command Flags
	Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "log_level",
			EnvVars: []string{"LOG_LEVEL"},
			Value:   "debug",
			Usage:   "Log Level",
		},
		// &cli.StringFlag{
		// 	Name:    "db_host",
		// 	EnvVars: []string{"DB_HOST"},
		// 	Usage:   "DB Host",
		// },
		// &cli.StringFlag{
		// 	Name:    "db_user",
		// 	EnvVars: []string{"DB_USER"},
		// 	Usage:   "DB User",
		// },
		// &cli.StringFlag{
		// 	Name:    "db_name",
		// 	EnvVars: []string{"DB_NAME"},
		// 	Usage:   "DB Name",
		// },
		// &cli.StringFlag{
		// 	Name:    "db_sslmode",
		// 	EnvVars: []string{"DB_SSLMODE"},
		// 	Value:   "disable",
		// 	Usage:   "DB SSL Mode",
		// },
		// &cli.StringFlag{
		// 	Name:    "db_password",
		// 	EnvVars: []string{"DB_PASSWORD"},
		// 	Usage:   "DB Password",
		// },WEBITEL_DBO_ADDRESS
		// &cli.Uint64Flag{
		// 	Name:    "conversation_timeout_sec",
		// 	EnvVars: []string{"CONVERSATION_TIMEOUT_SEC"},
		// 	Usage:   "Conversation timeout. sec",
		// },
		&cli.StringFlag{
			Name:    "db-dsn",
			EnvVars: []string{"WEBITEL_DBO_ADDRESS"},
			Usage:   "Persistent database driver name and a driver-specific data source name.",
		},
	}
)

func Run(ctx *cli.Context) error {

	service = micro.New(
		micro.Name(name),
		micro.Version(cmd.Version()),
		micro.WrapHandler(log.HandlerWrapper(&logger)),
		micro.WrapCall(log.CallWrapper(&logger)),
	)
	// Setup LOGs
	logs := "error"
	if ctx.IsSet("log_level") {
		logs = ctx.String("log_level")
	}
	colorize := true
	stdlog, err := log.Console(logs, colorize) // NewLogger(cfg.LogLevel)
	if err != nil {
		logger.Fatal().
			Str("app", "failed to parse log level").
			Msg(err.Error())
		return err
	}
	logger = *(stdlog)
	log.Default = *(stdlog)

	// Setup DSN; Persistant store
	dbo, err := OpenDB(ctx.String("db-dsn"))
	if err != nil {
		logger.Fatal().Err(err).Msg("[--db-dsn] Invalid DSN String")
		return err
	}

	// redisTable = c.String("store_table")
	timeout = 600 // c.Uint64("conversation_timeout_sec")
	store := pg.NewRepository(dbo, &logger)

	//cache := cache.NewChatCache(service.Options().Store)

	sender := wrapper.FromService(
		service.Name(), service.Server().Options().Id, service.Client(),
	)

	botClient = pbbot.NewBotsService("webitel.chat.bot", sender)
	// botClient = pbbot.NewBotsService("chat.bot", sender)
	authClient = pbauth.NewAuthService("go.webitel.app", sender)
	storageClient = pbstorage.NewFileService("storage", sender)
	flowClient = pbmanager.NewFlowChatServerService("workflow", sender) // wrapper.FromService(service.Name(), service.Server().Options().Id, service.Client()),

	flow := flow.NewClient(&logger, store, flowClient)
	auth := auth.NewClient(&logger, authClient)
	eventRouter := event.NewRouter(botClient /*flow,*/, broker.DefaultBroker, store, &logger)
	// serv := NewChatService(pgstore, repo, &logger, flow, auth, botClient, storageClient, eventRouter)
	serv := NewChatService(store, &logger, flow, auth, botClient, storageClient, eventRouter)

	if err := pb.RegisterChatServiceHandler(service.Server(), serv); err != nil {
		logger.Fatal().
			Str("app", "failed to register service").
			Msg(err.Error())
		return err
	}
	if err := pb.RegisterMessagesHandler(service.Server(), serv); err != nil {
		logger.Fatal().
			Str("app", "failed to register service").
			Msg(err.Error())
		return err
	}

	catalog := NewCatalog(
		CatalogLogs(&logger),
		CatalogAuthN(authN.NewClient(
			authN.ClientService(service),
			authN.ClientCache(authN.NewLru(4096)),
		)),
		CatalogStore(store),
	)

	if err := pb2.RegisterCatalogHandler(
		service.Server(), catalog,
	); err != nil {
		logger.Fatal().
			Str("app", "failed to register service").
			Msg(err.Error())
		return err
	}

	///debug/events
	///debug/requests
	httpsrv := http.Server{
		Addr: "127.0.0.1:6060",
	}
	go func() {
		_ = httpsrv.ListenAndServe()
	}()

	if err := service.Run(); err != nil {
		_ = httpsrv.Close()
		logger.Fatal().
			Str("app", "failed to run service").
			Msg(err.Error())
	}

	_ = httpsrv.Close()
	return nil
}

func init() {
	command := &cli.Command{
		Name:   "app",
		Usage:  usage,
		Flags:  Flags,
		Action: Run,
	}
	cmd.Register(command)
}
