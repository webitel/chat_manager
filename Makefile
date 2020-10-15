.PHONY: vendor

# download vendor
vendor:
	GO111MODULE=on go mod vendor

# start all unit tests
tests:
	go test ./...

# build chat service
build-chat:
	go build -mod=mod -o bin/webitel.chat.server ./cmd/chat/*.go

# build bot service
build-bot:
	go build -mod=mod -o bin/webitel.chat.bot ./cmd/bot/*.go

# build all servises
build: build-chat build-bot

# generate boiler models
generate-boiler:
	sqlboiler --wipe --no-tests -o ./models -c ./configs/sqlboiler.toml psql

proto:
	./scripts/protoc.sh

run-chat: build-chat
	./bin/webitel.chat.server --server_address=":9998" \
	--client_retries=0 \
	--client_request_timeout="1m" \
	--registry="consul" \
	--registry_address="consul" \
	--store="redis" \
	--store_table="chat:" \
	--store_address="redis:6379" \
	--broker="rabbitmq" \
	--broker_address="amqp://rabbitmq:rabbitmq@rabbitmq:5672/" \
	--webitel_dbo_address="postgres://postgres:postgres@postgres:5432/postgres?sslmode=disable" \
	--log_level="trace" \
	--conversation_timeout_sec=600

run-bot: build-bot
	./bin/webitel.chat.bot \
	--client_retries=0 \
	--client_request_timeout="1m" \
	--registry="consul" \
	--registry_address="consul" \
	--webhook_address="https://example.com/" \
	--app_port=8889
