#!/bin/sh
set -x

protoc -I api/proto/chat --go_out=api/proto/chat --micro_out=api/proto/chat api/proto/chat/chat.proto
mv ./api/proto/chat/github.com/webitel/chat_manager/api/proto/chat/* ./api/proto/chat/
rm -rf ./api/proto/chat/github.com

protoc -I api/proto/flow_manager --go_out=api/proto/flow_manager --micro_out=api/proto/flow_manager api/proto/flow_manager/flow_manager.proto

protoc -I api/proto/chat -I api/proto/bot --go_out=api/proto/bot --micro_out=api/proto/bot api/proto/bot/bot.proto
mv ./api/proto/bot/github.com/webitel/chat_manager/api/proto/bot/* ./api/proto/bot/
rm -rf ./api/proto/bot/github.com

protoc -I api/proto/auth --go_out=api/proto/auth --micro_out=api/proto/auth api/proto/auth/authN.proto

protoc -I api/proto/storage --go_out=api/proto/storage --micro_out=api/proto/storage api/proto/storage/file.proto
mv ./api/proto/storage/github.com/webitel/storage/grpc_api/storage/* ./api/proto/storage/
rm -rf ./api/proto/storage/github.com