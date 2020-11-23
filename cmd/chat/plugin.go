package main

import (

	"google.golang.org/grpc"
	// enable gRPC connectivity state
	_ "google.golang.org/grpc/channelz/service"

)

func init() {

	grpc.EnableTracing = true
}