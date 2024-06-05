#!/bin/sh
set -x

src=internal/repo/sqlx/proto
dst=$src

# ensure target dir exists
mkdir -p $dst

protoc -I $src -I proto \
  --go_opt=paths=source_relative --go_out=$dst \
  $src/*.proto