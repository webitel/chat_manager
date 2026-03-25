package chat

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	micro "github.com/micro/micro/v3/service"
	"github.com/micro/micro/v3/service/broker"
	"github.com/micro/micro/v3/service/server"
	microgrpcsrv "github.com/micro/micro/v3/service/server/grpc"
	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"google.golang.org/grpc"

	pbcontact "github.com/webitel/protos/gateway/contacts"
	otelsdk "github.com/webitel/webitel-go-kit/otel/sdk"

	pbauth "github.com/webitel/chat_manager/api/proto/auth"
	pbbot "github.com/webitel/chat_manager/api/proto/bot"
	pb "github.com/webitel/chat_manager/api/proto/chat"
	pb2 "github.com/webitel/chat_manager/api/proto/chat/messages"
	pbstorage "github.com/webitel/chat_manager/api/proto/storage"
	pbmanager "github.com/webitel/chat_manager/api/proto/workflow"
	authN "github.com/webitel/chat_manager/auth"
	"github.com/webitel/chat_manager/cmd"
	"github.com/webitel/chat_manager/internal/auth"
	event "github.com/webitel/chat_manager/internal/event_router"
	"github.com/webitel/chat_manager/internal/flow"
	pg "github.com/webitel/chat_manager/internal/repo/sqlx"
	"github.com/webitel/chat_manager/internal/wrapper"
	"github.com/webitel/chat_manager/log"
	"github.com/webitel/chat_manager/otel"
	"github.com/webitel/chat_manager/store/postgres"
)

const (
	name  = "webitel.chat.server" // "chat.srv"
	usage = "Run a chat messages service"

	webitelGo = "go.webitel.app"
)

var (
	service *micro.Service

	flowClient    pbmanager.FlowChatServerService
	botClient     pbbot.BotsService // pbbot.BotService
	authClient    pbauth.AuthService
	storageClient pbstorage.FileService
	timeout       uint64

	Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "log_level",
			EnvVars: []string{"WBTL_LOG_LEVEL"},
			Usage:   "Log Level",
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
		micro.Name(name),
		micro.Version(cmd.Version()),
		micro.WrapCall(log.CallWrapper(stdlog)),
		micro.WrapHandler(log.HandlerWrapper(stdlog)),
	)

	// Setup DSN; Persistant store
	dbopts := []postgres.Option{
		postgres.WithMaxOpenConns(ctx.Int("db-max-open-conns")),
		postgres.WithMaxIdleConns(ctx.Int("db-max-idle-conns")),
		postgres.WithConnMaxIdleTime(ctx.Duration("db-conn-max-idle-time")),
		postgres.WithConnMaxLifetime(ctx.Duration("db-conn-max-lifetime")),
	}

	dbo, err := OpenDB(stdlog, ctx.String("db-dsn"), dbopts...)
	if err != nil {
		log.FataLog(stdlog,
			"[--db-dsn] Invalid DSN String",
			slog.Any("error", err),
		)

		return err
	}

	// redisTable = c.String("store_table")
	timeout = 600 // c.Uint64("conversation_timeout_sec")
	store := pg.NewRepository(dbo, stdlog)

	// cache := cache.NewChatCache(service.Options().Store)

	sender := wrapper.FromService(
		service.Name(), service.Server().Options().Id, service.Client(),
	)
	// REdirect server requests !
	botClient = pbbot.NewBotsService("webitel.chat.bot", sender)
	authClient = pbauth.NewAuthService(webitelGo, sender)
	storageClient = pbstorage.NewFileService("storage", sender)
	flowClient = pbmanager.NewFlowChatServerService("workflow", sender) // wrapper.FromService(service.Name(), service.Server().Options().Id, service.Client()),

	imClientsClient := pbcontact.NewIMClientsService(webitelGo, sender)
	contactsClient := pbcontact.NewContactsService(webitelGo, sender)

	flow := flow.NewClient(stdlog, store, flowClient)
	auth := auth.NewClient(stdlog, authClient)
	eventRouter := event.NewRouter(botClient /*flow,*/, broker.DefaultBroker, store, stdlog)

	serv := NewChatService(store, stdlog, flow, auth, botClient, storageClient, eventRouter)

	for _, regErr := range []error{
		pb.RegisterChatServiceHandler(service.Server(), serv),
		pb.RegisterMessagesServiceHandler(service.Server(), serv),
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

	catalog := NewCatalog(
		CatalogLogs(stdlog),
		CatalogAuthN(authN.NewClient(
			authN.ClientService(service),
			authN.ClientCache(authN.NewLru(4096)),
		)),
		CatalogStore(store),
	)

	if err := pb2.RegisterCatalogHandler(
		service.Server(), catalog,
	); err != nil {
		log.FataLog(stdlog,
			"failed to register service",
			slog.Any("error", err),
		)
		return err
	}

	contactLinking := NewContactLinkingService(
		ContactLinkingServiceLogs(stdlog),
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
		AgentChatServiceLogs(stdlog),
		AgentChatServiceAuthN(authN.NewClient(
			authN.ClientService(service),
			authN.ClientCache(authN.NewLru(4096)),
		)),
		AgentChatServiceConversationStore(store),
	)

	if err := pb2.RegisterAgentChatServiceHandler(
		service.Server(), agentChatService,
	); err != nil {
		log.FataLog(stdlog,
			"failed to register service",
			slog.Any("error", err),
		)
		return err
	}

	contactChatHistory := NewContactChatHistoryService(
		ContactChatHistoryServiceLogs(stdlog),
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
		log.FataLog(stdlog,
			"failed to register service",
			slog.Any("error", err),
		)
		return err
	}

	if err := pb2.RegisterContactsChatCatalogHandler(
		service.Server(), contactChatHistory,
	); err != nil {
		log.FataLog(stdlog,
			"failed to register service",
			slog.Any("error", err),
		)
		return err
	}

	caseChatHistory := NewCaseChatHistoryService(
		CaseChatHistoryServiceLogs(stdlog),
		CaseChatHistoryServiceAuthN(authN.NewClient(
			authN.ClientService(service),
			authN.ClientCache(authN.NewLru(4096)),
		)),
		CaseChatHistoryServiceStore(store),
	)

	if err := pb2.RegisterCasesChatCatalogHandler(
		service.Server(), caseChatHistory,
	); err != nil {
		log.FataLog(stdlog,
			"failed to register service",
			slog.Any("error", err),
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
	defer httpsrv.Close()

	if err := service.Run(); err != nil {
		log.FataLog(stdlog,
			"failed to run service",
			slog.Any("error", err),
		)
		return err
	}

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
