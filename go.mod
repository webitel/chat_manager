module github.com/webitel/chat_manager

go 1.14

require (
	github.com/Masterminds/squirrel v1.5.0
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1
	github.com/golang/protobuf v1.4.3
	github.com/google/uuid v1.1.2
	github.com/gorilla/websocket v1.4.2
	github.com/jackc/pgconn v1.7.2
	github.com/jackc/pgtype v1.6.1
	github.com/jackc/pgx/v4 v4.9.2
	github.com/jmoiron/sqlx v1.2.0
	github.com/kr/pretty v0.2.0 // indirect
	github.com/micro/cli/v2 v2.1.2
	github.com/micro/go-micro/v2 v2.9.1
	github.com/micro/go-plugins/broker/rabbitmq/v2 v2.9.1
	github.com/micro/go-plugins/registry/consul/v2 v2.9.1
	github.com/mitchellh/mapstructure v1.3.3 // indirect
	github.com/pkg/errors v0.9.1
	github.com/rs/zerolog v1.19.0
	github.com/webitel/protos v1.0.0 // indirect
	github.com/webitel/protos/bot v0.0.0-20210728194921-d25cb1a4f895 // indirect
	github.com/webitel/protos/chat v0.0.0-20210728194921-d25cb1a4f895 // indirect
	github.com/webitel/protos/engine v0.0.0-20210118102359-591a476da972 // indirect
	github.com/webitel/protos/storage v0.0.0-20210118102359-591a476da972 // indirect
	github.com/webitel/protos/workflow v0.0.0-20210118102359-591a476da972 // indirect
	golang.org/x/crypto v0.0.0-20201016220609-9e8e0b390897 // indirect
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8
	google.golang.org/grpc v1.33.1
	google.golang.org/protobuf v1.25.0
//	google.golang.org/protobuf/cmd/protoc-gen-go v1.25.0
//	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.0.1
//	github.com/micro/micro/v2/cmd/protoc-gen-micro v2.9.3
)

replace (
	github.com/coreos/etcd => github.com/ozonru/etcd v3.3.20-grpc1.27-origmodule+incompatible
	// github.com/coreos/etcd => go.etcd.io/etcd v0.5.0-alpha.5.0.20200425165423-262c93980547
	// go.etcd.io/etcd => go.etcd.io/etcd v0.5.0-alpha.5.0.20200425165423-262c93980547
	google.golang.org/grpc => google.golang.org/grpc v1.27.0
)
