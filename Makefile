.PHONY: vendor

# non-falat if not exists
-include .env

# export
# GO111MODULE=on
# GOFLAGS=-mod=vendor

GOPKG=$(shell go list -m)
GOSRC=$(shell go list -f '{{.Dir}}' -m)
#GOSRC=$(shell go env GOMOD | xargs dirname)

export GOFLAGS=
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
	@echo GOFLAGS=$(shell go env GOFLAGS)

# download vendor
vendor:

	GO111MODULE=on \
	go mod vendor -v

proto:

	./scripts/protoc.sh

# start all unit tests
tests:
	go test ./...

# build chat service
build-chat: proto

	@echo "  >  Building binary: chat-srv"
	go build -mod=vendor -o chat-srv ./cmd/chat/*.go

# build bot service
build-bot: proto

	@echo "  >  Building binary: chat-bot"
	go build -mod=vendor -o chat-bot ./cmd/bot/*.go

# build all servises
build: vendor build-chat build-bot

# generate boiler models
generate-boiler:

	sqlboiler --wipe --no-tests -o ./models -c ./configs/sqlboiler.toml psql


.PHONY: chat-srv chat-bot

chat-srv: build-chat

	./chat-srv \
	--conversation_timeout_sec=600

chat-bot: build-bot

	./chat-bot \
	--broker= \
	--broker_address= \
	--address=:10128 \
	--site_url=https://example.com/chat \
	
