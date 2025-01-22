#!/bin/sh

src=proto/portal
dst=api/proto/portal

# ensure target dir exists
mkdir -p $dst

protoc -I $src -I proto \
  --go_opt=paths=source_relative --go_out=$dst \
  --micro_out=plugins=grpc,paths=source_relative:$dst \
  $src/*.proto