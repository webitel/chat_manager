package chat

import (
	micro "github.com/micro/micro/v3/service"
	"github.com/micro/micro/v3/service/broker"
	"github.com/micro/micro/v3/service/logger"
	"github.com/micro/micro/v3/service/server"
	"github.com/urfave/cli/v2"
	pbauth "github.com/webitel/chat_manager/api/proto/auth"
	pbbot "github.com/webitel/chat_manager/api/proto/bot"
	pb "github.com/webitel/chat_manager/api/proto/chat"
	pb2 "github.com/webitel/chat_manager/api/proto/chat/messages"
	pbstorage "github.com/webitel/chat_manager/api/proto/storage"
	pbmanager "github.com/webitel/chat_manager/api/proto/workflow"
	"github.com/webitel/chat_manager/cmd"
	"github.com/webitel/chat_manager/log"
	pbcontact "github.com/webitel/protos/gateway/contacts"
	slogutil "github.com/webitel/webitel-go-kit/otel/log/bridge/slog"
	otelsdk "github.com/webitel/webitel-go-kit/otel/sdk"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"log/slog"
	"net/http"
	"os"
	"strings"

	authN "github.com/webitel/chat_manager/auth"
	"github.com/webitel/chat_manager/internal/auth"
	event "github.com/webitel/chat_manager/internal/event_router"
	"github.com/webitel/chat_manager/internal/flow"
	pg "github.com/webitel/chat_manager/internal/repo/sqlx"
	"github.com/webitel/chat_manager/internal/wrapper"

	// -------------------- plugin(s) -------------------- //
	_ "github.com/webitel/webitel-go-kit/otel/sdk/log/otlp"
	_ "github.com/webitel/webitel-go-kit/otel/sdk/log/stdout"
	_ "github.com/webitel/webitel-go-kit/otel/sdk/metric/otlp"
	_ "github.com/webitel/webitel-go-kit/otel/sdk/metric/stdout"
	_ "github.com/webitel/webitel-go-kit/otel/sdk/trace/otlp"
	_ "github.com/webitel/webitel-go-kit/otel/sdk/trace/stdout"
)

const (
	name  = "webitel.chat.server" // "chat.srv"
	usage = "Run a chat messages service"

	webitelGo = "go.webitel.app"
)

var (
	service *micro.Service
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

	// Retrieve log level from the environment, default to info
	var verbose slog.LevelVar
	var slogger *slog.Logger
	verbose.Set(slog.LevelInfo)

	// TODO
	textLvl := os.Getenv("OTEL_LOG_LEVEL")
	if textLvl == "" {
		textLvl = os.Getenv("MICRO_LOG_LEVEL")
	}

	if textLvl != "" {
		_ = verbose.UnmarshalText([]byte(textLvl))
	}
	slogger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: verbose.Level()}))
	logger.DefaultLogger = log.NewSlogAdapter(slogger)

	version := cmd.Version()
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

	shutdown, err := otelsdk.Configure(ctx.Context,
		otelsdk.WithResource(resource.NewSchemaless(
			resourceAttrs...,
		)),
		otelsdk.WithLogBridge(func() {
			slogger = slog.New(
				slogutil.WithLevel(
					&verbose,                    // Filter level for otelslog.Handler
					otelslog.NewHandler("slog"), // otelslog Handler for OpenTelemetry
				),
			)
			slog.SetDefault(slogger)
		}),
	)

	if err != nil {
		return nil // log(FATAL) & exit(1)
	}
	defer func() {
		shutdown(ctx.Context)
	}()
	logger.DefaultLogger = log.NewSlogAdapter(slogger)

	service = micro.New(
		micro.Name(name),
		micro.Version(cmd.Version()),
		micro.WrapHandler(log.HandlerWrapper(slogger)),
		micro.WrapCall(log.CallWrapper(slogger)),
	)

	// Setup DSN; Persistant store
	dbo, err := OpenDB(slogger, ctx.String("db-dsn"))
	if err != nil {
		log.FataLog(slogger, err.Error(),
			slog.Any("error", err),
			slog.Any("app", "[--db-dsn] Invalid DSN String"),
		)
		return err
	}

	// redisTable = c.String("store_table")
	timeout = 600 // c.Uint64("conversation_timeout_sec")
	store := pg.NewRepository(dbo, slogger)

	//cache := cache.NewChatCache(service.Options().Store)

	sender := wrapper.FromService(
		service.Name(), service.Server().Options().Id, service.Client(),
	)
	// REdirect server requests !
	botClient = pbbot.NewBotsService("webitel.chat.bot", sender)
	// botClient = pbbot.NewBotsService("chat.bot", sender)
	authClient = pbauth.NewAuthService(webitelGo, sender)
	storageClient = pbstorage.NewFileService("storage", sender)
	flowClient = pbmanager.NewFlowChatServerService("workflow", sender) // wrapper.FromService(service.Name(), service.Server().Options().Id, service.Client()),

	imClientsClient := pbcontact.NewIMClientsService(webitelGo, sender)
	contactsClient := pbcontact.NewContactsService(webitelGo, sender)

	flow := flow.NewClient(slogger, store, flowClient)
	auth := auth.NewClient(slogger, authClient)
	eventRouter := event.NewRouter(botClient /*flow,*/, broker.DefaultBroker, store, slogger)
	// serv := NewChatService(pgstore, repo, &logger, flow, auth, botClient, storageClient, eventRouter)
	serv := NewChatService(store, slogger, flow, auth, botClient, storageClient, eventRouter)

	if err := pb.RegisterChatServiceHandler(service.Server(), serv); err != nil {
		log.FataLog(slogger, err.Error(),
			slog.Any("error", err),
			slog.Any("app", "failed to register service"),
		)
		return err
	}
	if err := pb.RegisterMessagesHandler(service.Server(), serv); err != nil {
		log.FataLog(slogger, err.Error(),
			slog.Any("error", err),
			slog.Any("app", "failed to register service"),
		)
		return err
	}

	catalog := NewCatalog(
		CatalogLogs(slogger),
		CatalogAuthN(authN.NewClient(
			authN.ClientService(service),
			authN.ClientCache(authN.NewLru(4096)),
		)),
		CatalogStore(store),
	)

	if err := pb2.RegisterCatalogHandler(
		service.Server(), catalog,
	); err != nil {
		slogger.Error(err.Error(),
			slog.Any("error", err),
			slog.String("app", "failed to register service"),
		)
		return err
	}

	contactLinking := NewContactLinkingService(
		ContactLinkingServiceLogs(slogger),
		ContactLinkingServiceAuthN(authN.NewClient(
			authN.ClientService(service),
			authN.ClientCache(authN.NewLru(4096)),
		)),
		ContactLinkingServiceChannelStore(store),
		ContactLinkingServiceClientStore(store),
		ContactsLinkingServiceIMClient(imClientsClient),
		ContactsLinkingServiceContactClient(contactsClient),
	)

	agentChatService := NewAgentChatService(
		AgentChatServiceLogs(slogger),
		AgentChatServiceAuthN(authN.NewClient(
			authN.ClientService(service),
			authN.ClientCache(authN.NewLru(4096)),
		)),
		AgentChatServiceConversationStore(store),
	)

	if err := pb2.RegisterAgentChatServiceHandler(
		service.Server(), agentChatService,
	); err != nil {
		log.FataLog(slogger, err.Error(),
			slog.Any("error", err),
			slog.Any("app", "failed to register service"),
		)
		return err
	}

	contactChatHistory := NewContactChatHistoryService(
		ContactChatHistoryServiceLogs(slogger),
		ContactChatHistoryServiceAuthN(authN.NewClient(
			authN.ClientService(service),
			authN.ClientCache(authN.NewLru(4096)),
		)),
		ContactChatHistoryServiceStore(store),
		ContactChatHistoryServiceContactClient(contactsClient),
	)

	if err := pb2.RegisterContactLinkingServiceHandler(
		service.Server(), contactLinking,
	); err != nil {
		log.FataLog(slogger, err.Error(),
			slog.Any("error", err),
			slog.Any("app", "failed to register service"),
		)
		return err
	}

	if err := pb2.RegisterContactsChatCatalogHandler(
		service.Server(), contactChatHistory,
	); err != nil {
		log.FataLog(slogger, err.Error(),
			slog.Any("error", err),
			slog.Any("app", "failed to register service"),
		)
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
		log.FataLog(slogger, err.Error(),
			slog.Any("error", err),
			slog.Any("app", "failed to run servic"),
		)
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
