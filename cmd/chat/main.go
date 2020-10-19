package main

import (
	"os"

	pbauth "github.com/webitel/chat_manager/api/proto/auth"
	pbbot "github.com/webitel/chat_manager/api/proto/bot"
	pb "github.com/webitel/chat_manager/api/proto/chat"
	pbmanager "github.com/webitel/chat_manager/api/proto/flow_manager"
	pbstorage "github.com/webitel/chat_manager/api/proto/storage"
	"github.com/webitel/chat_manager/internal/auth"
	cache "github.com/webitel/chat_manager/internal/chat_cache"
	event "github.com/webitel/chat_manager/internal/event_router"
	"github.com/webitel/chat_manager/internal/flow"
	pg "github.com/webitel/chat_manager/internal/repo/sqlx"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/micro/cli/v2"
	"github.com/micro/go-micro/v2"
	"github.com/micro/go-micro/v2/config/cmd"
	"github.com/micro/go-micro/v2/store"
	"github.com/micro/go-plugins/broker/rabbitmq/v2"
	"github.com/micro/go-plugins/registry/consul/v2"
	"github.com/micro/go-plugins/store/redis/v2"
	"github.com/rs/zerolog"
)

type Config struct {
	LogLevel string
	DBSource string
}

var (
	logger     *zerolog.Logger
	cfg        *Config
	service    micro.Service
	redisStore store.Store
	// rabbitBroker broker.Broker
	redisTable    string
	flowClient    pbmanager.FlowChatServerService
	botClient     pbbot.BotService
	authClient    pbauth.AuthService
	storageClient pbstorage.FileService
	timeout       uint64
)

func init() {
	// plugins
	cmd.DefaultBrokers["rabbitmq"] = rabbitmq.NewBroker
	cmd.DefaultStores["redis"] = redis.NewStore
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
	)
	service.Init(
		micro.Action(func(c *cli.Context) error {
			cfg.LogLevel = c.String("log_level")
			cfg.DBSource = c.String("webitel_dbo_address")
			redisTable = c.String("store_table")
			timeout = 600 //c.Uint64("conversation_timeout_sec")
			var err error
			logger, err = NewLogger(cfg.LogLevel)
			if err != nil {
				logger.Fatal().
					Str("app", "failed to parse log level").
					Msg(err.Error())
				return err
			}
			flowClient = pbmanager.NewFlowChatServerService("workflow", service.Client())
			botClient = pbbot.NewBotService("webitel.chat.bot", service.Client())
			authClient = pbauth.NewAuthService("go.webitel.app", service.Client())
			storageClient = pbstorage.NewFileService("storage", service.Client())
			return nil
		}),
		micro.Broker(
			rabbitmq.NewBroker(
				rabbitmq.ExchangeName("chat"),
				rabbitmq.DurableExchange(),
			),
		),
	)

	service.Options().Store.Init(store.Table(redisTable))

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

	db, err := sqlx.Open("postgres", cfg.DBSource) //DbSource(cfg.DBHost, cfg.DBUser, cfg.DBName, cfg.DBPassword, cfg.DBSSLMode))
	if err != nil {
		logger.Fatal().
			Str("app", "failed to connect db").
			Msg(err.Error())
		return
	}

	logger.Debug().
		Str("cfg.DBSource", cfg.DBSource).
		Msg("db connected")

	repo := pg.NewRepository(db, logger)
	cache := cache.NewChatCache(service.Options().Store)
	flow := flow.NewClient(logger, flowClient, cache)
	auth := auth.NewClient(logger, cache, authClient)
	eventRouter := event.NewRouter(botClient /*flow,*/, service.Options().Broker, repo, logger)
	serv := NewChatService(repo, logger, flow, auth, botClient, storageClient, cache, eventRouter)

	if err := pb.RegisterChatServiceHandler(service.Server(), serv); err != nil {
		logger.Fatal().
			Str("app", "failed to register service").
			Msg(err.Error())
		return
	}

	if err := service.Run(); err != nil {
		logger.Fatal().
			Str("app", "failed to run service").
			Msg(err.Error())
	}
}

// func DbSource(host, user, dbName, password, sslMode string) string {
// 	dbinfo := fmt.Sprintf("host=%s user=%s dbname=%s sslmode=%s", host, user, dbName, sslMode)
// 	if password != "" {
// 		dbinfo += fmt.Sprintf(" password=%s", password)
// 	}
// 	return dbinfo
// }

func NewLogger(logLevel string) (*zerolog.Logger, error) {
	lvl, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		return nil, err
	}

	l := zerolog.New(os.Stdout).With().Timestamp().Logger()
	l = l.Level(lvl)

	return &l, nil
}
