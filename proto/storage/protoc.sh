#!/bin/sh

src=proto/storage
dst=api/proto/storage

# ensure target dir exists
mkdir -p $dst

protoc -I $src -I proto \
  --go_opt=paths=source_relative --go_out=$dst \
  --micro_out=plugins=grpc,paths=source_relative:$dst \
  $src/*.proto