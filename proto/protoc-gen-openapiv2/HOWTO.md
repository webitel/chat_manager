
## **Install** [protoc-gen-openapiv2](https://github.com/grpc-ecosystem/grpc-gateway/tree/main/protoc-gen-openapiv2)
```sh
> go get -v github.com/grpc-ecosystem/grpc-gateway/v2
> go install -v github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2
```

## **Plugin** [Options](https://github.com/grpc-ecosystem/grpc-gateway/blob/main/protoc-gen-openapiv2/main.go#L18).

```
protoc -I proto \
  --openapiv2_out=allow_merge,merge_file_name=$out_filename:$dst \
  $src/*.proto
```

## **Web** [Editor](https://editor-next.swagger.io/).
