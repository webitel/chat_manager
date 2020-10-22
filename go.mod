module github.com/webitel/chat_manager

go 1.14

//replace (
//	github.com/coreos/etcd => go.etcd.io/etcd v0.5.0-alpha.5.0.20200425165423-262c93980547
//	go.etcd.io/etcd => go.etcd.io/etcd v0.5.0-alpha.5.0.20200425165423-262c93980547
//)

require (
	github.com/go-telegram-bot-api/telegram-bot-api v4.6.4+incompatible
	github.com/gogo/protobuf v1.2.2-0.20190723190241-65acae22fc9d // indirect
	github.com/golang/protobuf v1.4.3
	github.com/google/uuid v1.1.2
	github.com/gorilla/mux v1.7.3
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/jmoiron/sqlx v1.2.0
	github.com/kr/pretty v0.2.0 // indirect
	github.com/lib/pq v1.8.0
	github.com/micro/cli/v2 v2.1.2
	github.com/micro/go-micro/v2 v2.9.1
	github.com/micro/go-plugins/broker/rabbitmq/v2 v2.9.1
	github.com/micro/go-plugins/registry/consul/v2 v2.9.1
	github.com/micro/go-plugins/store/redis/v2 v2.9.1
	github.com/mitchellh/mapstructure v1.3.3 // indirect
	github.com/rs/zerolog v1.19.0
	github.com/webitel/protos/pkg/bot v0.0.0-20201022120304-7cd198480a6d
	github.com/webitel/protos/pkg/chat v0.0.0-20201022120304-7cd198480a6d
	github.com/webitel/protos/pkg/workflow v0.0.0-20201022120304-7cd198480a6d
	golang.org/x/crypto v0.0.0-20201016220609-9e8e0b390897 // indirect
	google.golang.org/grpc v1.33.1 // indirect
	google.golang.org/protobuf v1.25.0
	gopkg.in/yaml.v2 v2.2.8 // indirect
)

replace (
	github.com/coreos/etcd => github.com/ozonru/etcd v3.3.20-grpc1.27-origmodule+incompatible
	//github.com/coreos/etcd => go.etcd.io/etcd v0.5.0-alpha.5.0.20200425165423-262c93980547
	//github.com/webitel/protos/pkg/bot => ../protos/pkg/bot
	//github.com/webitel/protos/pkg/chat => ../protos/pkg/chat
	//github.com/webitel/protos/pkg/workflow => ../protos/pkg/workflow
	//go.etcd.io/etcd => go.etcd.io/etcd v0.5.0-alpha.5.0.20200425165423-262c93980547
	google.golang.org/grpc => google.golang.org/grpc v1.27.0
)
