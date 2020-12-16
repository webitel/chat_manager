# non-falat if not exists
-include .env

GO111MODULE="on"

GOPKG=$(shell go list -m)
GOSRC=$(shell go list -f '{{.Dir}}' -m)
#GOSRC=$(shell go env GOMOD | xargs dirname)

GOFLAGS=$(shell go env GOFLAGS)
# Go related variables.
#GOBASE=GOSRC
GOPATH=$(shell go env GOPATH)
#GOPATH:="$(GOPATH):$(GOSRC)/vendor:$(GOSRC)"
#GOBIN=$(GOBASE)/bin
#GOFILES=$(wildcard *.go)
# $(GOSRC)/vendor

env:

	@echo GOPKG=$(GOPKG)
	@echo GOSRC=$(GOSRC)
	@echo GOPATH=$(GOPATH)
	@echo GOFLAGS=$(GOFLAGS)

protoc-gen-micro: $(GOPATH)/bin/protoc-gen-micro
	@cd ~
	GO111MODULE=on go get -v \
	github.com/micro/micro/v2/cmd/protoc-gen-micro@v2.9.1
	@cd -

protoc-gen-go: $(GOPATH)/bin/protoc-gen-go
	@cd ~
	GO111MODULE=on go get -v \
	google.golang.org/protobuf/cmd/protoc-gen-go@v1.25.0 \
	google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.0.1
	@cd -

# ensure protoc-gen-{plugins} version
protoc: protoc-gen-go protoc-gen-micro
	protoc --version

# %.proto: # expect that this touch all *.proto files; does not work
# 	sed -i 's/github.com\/webitel\/protos\//github.com\/webitel\/chat_manager\/api\/proto\//g' $@

# source *.proto files
api/proto/auth/%.proto:
	@sed -i 's/github.com\/webitel\/protos\//github.com\/webitel\/chat_manager\/api\/proto\//g' api/proto/auth/$*.proto

api/proto/auth/%.pb.micro.go: api/proto/auth/%.proto
	protoc -I api/proto/auth \
	--go_opt=paths=source_relative --go_out=api/proto/auth \
	--micro_out=plugins=grpc,paths=source_relative:api/proto/auth \
	$?

# source *.proto files
api/proto/chat/%.proto: $(shell go list -m -f {{.Dir}} github.com/webitel/protos/chat)/%.proto
	@mkdir -p api/proto/chat
	@cp -f -t api/proto/chat $?
	@sed -i 's/github.com\/webitel\/protos\//github.com\/webitel\/chat_manager\/api\/proto\//g' api/proto/chat/$*.proto

api/proto/chat/%.pb.micro.go: api/proto/chat/%.proto
	protoc -I api/proto/chat \
	--go_opt=paths=source_relative --go_out=api/proto/chat \
	--micro_out=plugins=grpc,paths=source_relative:api/proto/chat \
	api/proto/chat/*.proto

# source *.proto files
api/proto/bot/%.proto: $(shell go list -m -f {{.Dir}} github.com/webitel/protos/bot)/%.proto
	@mkdir -p api/proto/bot
	@cp -f -t api/proto/bot $?
	@sed -i 's/github.com\/webitel\/protos\//github.com\/webitel\/chat_manager\/api\/proto\//g' api/proto/bot/$*.proto

api/proto/bot/%.pb.micro.go: api/proto/bot/%.proto api/proto/chat/chat.proto
	protoc -I api/proto/bot -I api/proto/chat \
	--go_opt=paths=source_relative --go_out=api/proto/bot \
	--micro_out=plugins=grpc,paths=source_relative:api/proto/bot \
	api/proto/bot/*.proto

# source *.proto files
api/proto/storage/%.proto: $(shell go list -m -f {{.Dir}} github.com/webitel/protos/storage)/%.proto
	@mkdir -p api/proto/storage
	@cp -f -t api/proto/storage $?
	@sed -i 's/github.com\/webitel\/protos\//github.com\/webitel\/chat_manager\/api\/proto\//g' api/proto/storage/$*.proto

api/proto/storage/%.pb.micro.go: api/proto/storage/%.proto
	protoc -I api/proto/storage -I api/proto \
	--go_opt=paths=source_relative --go_out=api/proto/storage \
	--micro_out=plugins=grpc,paths=source_relative:api/proto/storage \
	api/proto/storage/*.proto

# source *.proto files
api/proto/workflow/%.proto: $(shell go list -m -f {{.Dir}} github.com/webitel/protos/workflow)/%.proto
	@mkdir -p api/proto/workflow
	@cp -f -t api/proto/workflow $?
	@sed -i 's/github.com\/webitel\/protos\//github.com\/webitel\/chat_manager\/api\/proto\//g' api/proto/workflow/$*.proto

api/proto/workflow/%.pb.micro.go: api/proto/workflow/%.proto
	protoc -I api/proto/workflow \
	--go_opt=paths=source_relative --go_out=api/proto/workflow \
	--micro_out=plugins=grpc,paths=source_relative:api/proto/workflow \
	api/proto/workflow/*.proto

# proto: protoc protoc-gen-go protoc-gen-micro
proto: \
api/proto/workflow/chat.proto api/proto/workflow/chat.pb.micro.go \
api/proto/storage/file.proto api/proto/storage/file.pb.micro.go \
api/proto/chat/chat.v1.proto api/proto/chat/chat.v1.pb.micro.go \
api/proto/chat/chat.proto api/proto/chat/chat.pb.micro.go \
api/proto/bot/bot.proto api/proto/bot/bot.pb.micro.go \
api/proto/auth/authN.pb.micro.go

clean-proto:
	rm -f api/proto/auth/*.pb.go api/proto/auth/*.pb.micro.go
	rm -rf api/proto/bot
	rm -rf api/proto/chat
	rm -rf api/proto/storage
	rm -rf api/proto/workflow

clean: clean-proto
	rm chat-srv chat-bot

# start all unit tests
tests:
	go test ./...

chat-srv: proto
	@echo "  >  Building binary: chat-srv"
	go build -o chat-srv ./cmd/chat/*.go

# build chat service
build-chat: chat-srv

chat-bot: proto
	@echo "  >  Building binary: chat-bot"
	go build -o chat-bot ./cmd/bot/*.go

# build bot service
build-bot: chat-bot

# build all servises
build: chat-srv chat-bot

# generate boiler models
generate-boiler:

	sqlboiler --wipe --no-tests -o ./models -c ./configs/sqlboiler.toml psql


.PHONY: server gateway

server: build-chat

	./chat-srv \
	--conversation_timeout_sec=600

gateway: build-bot

	./chat-bot \
	--broker= \
	--broker_address= \
	--address=0.0.0.0:10128 \
	--site_url=https://example.com/chat \
	
