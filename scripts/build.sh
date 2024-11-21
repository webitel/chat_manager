#!/bin/sh

set -x # verbose

# CGO_ENABLED=0 \
# GO111MODULE=on \
# go mod download

NAME=messages$(go env GOEXE)

GIT_TAG=$(git describe --abbrev=0 --tags --always --match "v*") # latest tag
GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD) # branch name
GIT_COMMIT=$(git rev-parse --short=12 HEAD) # commit hash
BUILD_DATE=$(date -u "+%Y%m%d%H%M%S")

GIT_IMPORT='github.com/webitel/chat_manager/cmd'
LDFLAGS="-X $GIT_IMPORT.GitCommit=$GIT_COMMIT -X $GIT_IMPORT.GitTag=$GIT_TAG -X $GIT_IMPORT.BuildDate=$BUILD_DATE"

CGO_ENABLED=0 \
GO111MODULE=on \
go build -a -installsuffix cgo -ldflags "-s -w ${LDFLAGS}" -o $NAME *.go