package main

import (
	// "github.com/webitel/chat_manager/internal/repo/postgres"
	// "github.com/webitel/chat_manager/internal/repo/store"


	"net/http"
	_ "net/http/pprof"
	
	"os"

	"github.com/rs/zerolog"
	"github.com/webitel/chat_manager/log"
	"github.com/webitel/chat_manager/internal/wrapper"

	pb "github.com/webitel/protos/chat"
	pbbot "github.com/webitel/protos/bot"
	pbauth "github.com/webitel/chat_manager/api/proto/auth" // "github.com/webitel/protos/auth"
	pbmanager "github.com/webitel/protos/workflow"
	// import go_package= proto definition option
	pbstorage "github.com/webitel/chat_manager/api/proto/storage"
	// ----- service clients -----
	"github.com/webitel/chat_manager/internal/auth"
	event "github.com/webitel/chat_manager/internal/event_router"
	"github.com/webitel/chat_manager/internal/flow"
	pg "github.com/webitel/chat_manager/internal/repo/sqlx"

	// "github.com/jmoiron/sqlx"
	// _ "github.com/lib/pq"
	"github.com/micro/cli/v2"
	"github.com/micro/go-micro/v2"
	"github.com/micro/go-micro/v2/config/cmd"
	"github.com/micro/go-plugins/broker/rabbitmq/v2"
	"github.com/micro/go-plugins/registry/consul/v2"
)

type Config struct {
	LogLevel string
	DBSource string
}

var (
	logger  = zerolog.New(os.Stdout)
	cfg     *Config
	service micro.Service
	//redisStore store.Store
	// rabbitBroker broker.Broker
	//redisTable    string
	flowClient    pbmanager.FlowChatServerService
	botClient     pbbot.BotService
	authClient    pbauth.AuthService
	storageClient pbstorage.FileService
	timeout       uint64
)

func init() {
	// plugins
	cmd.DefaultBrokers["rabbitmq"] = rabbitmq.NewBroker
	//cmd.DefaultStores["redis"] = redis.NewStore
	cmd.DefaultRegistries["consul"] = consul.NewRegistry
}

func main() {
	cfg = &Config{}
	service = micro.NewService(
		micro.Name("webitel.chat.server"),
		micro.Version("latest"),
		micro.Flags(
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
			&cli.Uint64Flag{
				Name:    "conversation_timeout_sec",
				EnvVars: []string{"CONVERSATION_TIMEOUT_SEC"},
				Usage:   "Conversation timeout. sec",
			},
			&cli.StringFlag{
				Name:    "webitel_dbo_address",
				EnvVars: []string{"WEBITEL_DBO_ADDRESS"},
				Value:   "disable",
				Usage:   "DB Connection string",
			},
		),
		micro.WrapHandler(log.HandlerWrapper(&logger)),
		micro.WrapCall(log.CallWrapper(&logger)),
	)
	service.Init(
		micro.Action(func(c *cli.Context) error {
			cfg.LogLevel = c.String("log_level")
			cfg.DBSource = c.String("webitel_dbo_address")
			//redisTable = c.String("store_table")
			timeout = 600 //c.Uint64("conversation_timeout_sec")
			var err error
			stdlog, err := log.Console(cfg.LogLevel, true) // NewLogger(cfg.LogLevel)
			if err != nil {
				logger.Fatal().
					Str("app", "failed to parse log level").
					Msg(err.Error())
				return err
			}
			logger = *(stdlog)
			log.Default = *(stdlog)
			return nil
		}),
		micro.Broker(
			rabbitmq.NewBroker(
				rabbitmq.ExchangeName("chat"),
				rabbitmq.DurableExchange(),
			),
		),
		// micro.AfterStart(func() error {
		// 	return http.ListenAndServe("localhost:6060", nil)
		// }),
	)

	//service.Options().Store.Init(store.Table(redisTable))

	if err := service.Options().Broker.Init(); err != nil {
		logger.Fatal().
			Str("app", "failed to init broker").
			Msg(err.Error())
		return
	}
	if err := service.Options().Broker.Connect(); err != nil {
		logger.Fatal().
			Str("app", "failed to connect broker").
			Msg(err.Error())
		return
	}
	

	db, err := OpenDB(cfg.DBSource)
	if err != nil {
		logger.Fatal().
			Str("app", "failed to connect db"). // This is NOT a connect; just DSN validation !
			Msg(err.Error())
		return
	}

	logger.Debug().
		Str("cfg.DBSource", cfg.DBSource).
		Msg("db connected")

	// v1: chain .this db transaction(s)
	// service.Init(micro.WrapHandler(
	// 	store.WrapDBSession(db.DB),
	// ))
	// v1
	// pgstore := postgres.NewChatStore(db, &logger)
	// v0
	repo := pg.NewRepository(db, &logger)

	//cache := cache.NewChatCache(service.Options().Store)
	
	botClient = pbbot.NewBotService("webitel.chat.bot", service.Client())
	authClient = pbauth.NewAuthService("go.webitel.app", service.Client())
	storageClient = pbstorage.NewFileService("storage", service.Client())
	flowClient = pbmanager.NewFlowChatServerService("workflow",
		wrapper.FromServiceId(service.Server().Options().Id, service.Client()),
	)

	flow := flow.NewClient(&logger, repo, flowClient)
	auth := auth.NewClient(&logger, authClient)
	eventRouter := event.NewRouter(botClient /*flow,*/, service.Options().Broker, repo, &logger)
	// serv := NewChatService(pgstore, repo, &logger, flow, auth, botClient, storageClient, eventRouter)
	serv := NewChatService(repo, &logger, flow, auth, botClient, storageClient, eventRouter)

	if err := pb.RegisterChatServiceHandler(service.Server(), serv); err != nil {
		logger.Fatal().
			Str("app", "failed to register service").
			Msg(err.Error())
		return
	}
	///debug/events
	///debug/requests
	httpsrv := http.Server{
		Addr: "127.0.0.1:6060",
	}
	go func() {
		httpsrv.ListenAndServe()
	} ()

	if err := service.Run(); err != nil {
		logger.Fatal().
			Str("app", "failed to run service").
			Msg(err.Error())
	}

	httpsrv.Close()
}
