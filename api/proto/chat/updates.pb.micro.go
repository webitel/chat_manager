// Code generated by protoc-gen-micro. DO NOT EDIT.
// source: updates.proto

package chat

import (
	fmt "fmt"
	proto "google.golang.org/protobuf/proto"
	_ "google.golang.org/protobuf/types/known/timestamppb"
	math "math"
)

import (
	context "context"
	api "github.com/micro/micro/v3/service/api"
	client "github.com/micro/micro/v3/service/client"
	server "github.com/micro/micro/v3/service/server"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// Reference imports to suppress errors if they are not otherwise used.
var _ api.Endpoint
var _ context.Context
var _ client.Option
var _ server.Option

// Api Endpoints for Updates service

func NewUpdatesEndpoints() []*api.Endpoint {
	return []*api.Endpoint{}
}

// Client API for Updates service

type UpdatesService interface {
	// OnUpdate message event handler
	OnUpdate(ctx context.Context, in *Update, opts ...client.CallOption) (*ACK, error)
}

type updatesService struct {
	c    client.Client
	name string
}

func NewUpdatesService(name string, c client.Client) UpdatesService {
	return &updatesService{
		c:    c,
		name: name,
	}
}

func (c *updatesService) OnUpdate(ctx context.Context, in *Update, opts ...client.CallOption) (*ACK, error) {
	req := c.c.NewRequest(c.name, "Updates.OnUpdate", in)
	out := new(ACK)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for Updates service

type UpdatesHandler interface {
	// OnUpdate message event handler
	OnUpdate(context.Context, *Update, *ACK) error
}

func RegisterUpdatesHandler(s server.Server, hdlr UpdatesHandler, opts ...server.HandlerOption) error {
	type updates interface {
		OnUpdate(ctx context.Context, in *Update, out *ACK) error
	}
	type Updates struct {
		updates
	}
	h := &updatesHandler{hdlr}
	return s.Handle(s.NewHandler(&Updates{h}, opts...))
}

type updatesHandler struct {
	UpdatesHandler
}

func (h *updatesHandler) OnUpdate(ctx context.Context, in *Update, out *ACK) error {
	return h.UpdatesHandler.OnUpdate(ctx, in, out)
}
