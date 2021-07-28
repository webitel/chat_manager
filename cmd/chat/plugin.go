package main

import (
	"google.golang.org/grpc"
	// enable gRPC connectivity state
	_ "google.golang.org/grpc/channelz/service"

	_ "github.com/micro/go-plugins/broker/rabbitmq/v2"
	_ "github.com/micro/go-plugins/registry/consul/v2"
)

func init() {

	grpc.EnableTracing = true
}