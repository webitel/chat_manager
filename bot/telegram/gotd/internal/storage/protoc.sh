#!/bin/sh

pkg=storage
src=bot/telegram/client/internal/$pkg
dst=bot/telegram/client/internal/$pkg

# ensure target dir exists
mkdir -p $dst

protoc -I $src -I proto \
  --go_opt=paths=source_relative --go_out=$dst \
  $src/*.proto