#!/bin/bash

# verbose
#set -x

# GO111MODULE=on
#gopath=$(go env GOMOD | xargs dirname)
gopath=$(go env GOPATH)

# for dep in  google.golang.org/protobuf/cmd/protoc-gen-go \
#             google.golang.org/grpc/cmd/protoc-gen-go-grpc \
#             github.com/micro/micro/v2/cmd/protoc-gen-micro
# do

#     go list -m $dep

# done

# GOFLAGS=-mod= \
go list -f '{{.ImportPath}}: {{.Module.Version}}' -mod= \
google.golang.org/protobuf/cmd/protoc-gen-go \
google.golang.org/grpc/cmd/protoc-gen-go-grpc \
github.com/micro/micro/v2/cmd/protoc-gen-micro
# echo protoc: $(protoc --version)
# echo protoc-gen-go: $(protoc-gen-go --version)
# # echo protoc-gen-go-grpc: $(protoc-gen-go-grpc --version) # NOT-applicable
# # echo protoc-gen-micro: $(protoc-gen-micro --version) # NOT-applicable

# enable go-vendor
#GOFLAGS=-mod=vendor
# enable go-modules
#GOFLAGS=-mod=
gopkg='go list -f ''{{.Dir}}'' -mod='
proto_path="github.com/webitel/protos"
dist=(chat bot)

# Regenerate module(s) protos
for i in ${!dist[@]}
do

    dist[$i]=$(${gopkg} ${proto_path}/${dist[$i]})

done

# Target locations
gosrc=$(go env GOMOD | xargs dirname) #$(${gopkg}) #$(${gopkg} .) #directory
proto=$gosrc/proto
vendor=$gosrc/vendor
#vendor=$gopath/pkg/mod

go_out=$vendor

# Ensure redistributed ./vendor directory exists !
mkdir -p $go_out
# Import -I --proto_path parameter(s) ...
proto_import=$(printf ' -I %s' "${dist[@]}")

for src in ${dist[@]}
do

    echo "protogen: ${src}/*.proto => $go_out"

    protoc $proto_import \
    --go_out=$go_out \
    --micro_out=$go_out \
     \
    $src/*.proto

done

# proto-auth; --go_opt=paths=source_relative
proto_pkg="api/proto/auth"
proto_path=$gosrc/$proto_pkg

echo "protogen: ${proto_path}/*.proto => --go_opt=paths=source_relative --go_out=${proto_pkg}"

protoc \
-I $proto_path \
--go_opt=paths=source_relative --go_out=${proto_pkg} \
--micro_out=plugins=grpc,paths=source_relative:${proto_pkg} \
 \
$proto_path/*.proto



# proto-storage; go_out=$vendor
proto_pkg="api/proto/storage"
proto_path=$gosrc/$proto_pkg

echo "protogen: ${proto_path}/*.proto => $go_out"

protoc \
-I $proto_path \
--go_opt=paths=source_relative --go_out=${proto_pkg} \
--micro_out=plugins=grpc,paths=source_relative:${proto_pkg} \
 \
$proto_path/*.proto