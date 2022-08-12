// Code generated by protoc-gen-micro. DO NOT EDIT.
// source: messages.proto

package chat

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	_ "google.golang.org/genproto/googleapis/rpc/status"
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

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

// Reference imports to suppress errors if they are not otherwise used.
var _ api.Endpoint
var _ context.Context
var _ client.Option
var _ server.Option

// Api Endpoints for Messages service

func NewMessagesEndpoints() []*api.Endpoint {
	return []*api.Endpoint{}
}

// Client API for Messages service

type MessagesService interface {
	// Broadcast message `from` given bot profile to `peer` recipient(s)
	BroadcastMessage(ctx context.Context, in *BroadcastMessageRequest, opts ...client.CallOption) (*BroadcastMessageResponse, error)
}

type messagesService struct {
	c    client.Client
	name string
}

func NewMessagesService(name string, c client.Client) MessagesService {
	return &messagesService{
		c:    c,
		name: name,
	}
}

func (c *messagesService) BroadcastMessage(ctx context.Context, in *BroadcastMessageRequest, opts ...client.CallOption) (*BroadcastMessageResponse, error) {
	req := c.c.NewRequest(c.name, "Messages.BroadcastMessage", in)
	out := new(BroadcastMessageResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for Messages service

type MessagesHandler interface {
	// Broadcast message `from` given bot profile to `peer` recipient(s)
	BroadcastMessage(context.Context, *BroadcastMessageRequest, *BroadcastMessageResponse) error
}

func RegisterMessagesHandler(s server.Server, hdlr MessagesHandler, opts ...server.HandlerOption) error {
	type messages interface {
		BroadcastMessage(ctx context.Context, in *BroadcastMessageRequest, out *BroadcastMessageResponse) error
	}
	type Messages struct {
		messages
	}
	h := &messagesHandler{hdlr}
	return s.Handle(s.NewHandler(&Messages{h}, opts...))
}

type messagesHandler struct {
	MessagesHandler
}

func (h *messagesHandler) BroadcastMessage(ctx context.Context, in *BroadcastMessageRequest, out *BroadcastMessageResponse) error {
	return h.MessagesHandler.BroadcastMessage(ctx, in, out)
}