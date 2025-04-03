// Code generated by protoc-gen-micro. DO NOT EDIT.
// source: messages.proto

package chat

import (
	fmt "fmt"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/options"
	messages "github.com/webitel/chat_manager/api/proto/chat/messages"
	_ "google.golang.org/genproto/googleapis/api/annotations"
	proto "google.golang.org/protobuf/proto"
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

// Api Endpoints for MessagesService service

func NewMessagesServiceEndpoints() []*api.Endpoint {
	return []*api.Endpoint{
		{
			Name:    "MessagesService.BroadcastMessage",
			Path:    []string{"/chat/broadcast"},
			Method:  []string{"POST"},
			Handler: "rpc",
		},
	}
}

// Client API for MessagesService service

type MessagesService interface {
	// Sends a current user action event to a conversation partners.
	SendUserAction(ctx context.Context, in *SendUserActionRequest, opts ...client.CallOption) (*SendUserActionResponse, error)
	// Broadcast message send message from via to peer recipients.
	BroadcastMessage(ctx context.Context, in *messages.BroadcastMessageRequest, opts ...client.CallOption) (*messages.BroadcastMessageResponse, error)
	// Broadcast message send message from via to peer recipients (for internal services).
	BroadcastMessageNA(ctx context.Context, in *messages.BroadcastMessageRequest, opts ...client.CallOption) (*messages.BroadcastMessageResponse, error)
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

func (c *messagesService) SendUserAction(ctx context.Context, in *SendUserActionRequest, opts ...client.CallOption) (*SendUserActionResponse, error) {
	req := c.c.NewRequest(c.name, "MessagesService.SendUserAction", in)
	out := new(SendUserActionResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *messagesService) BroadcastMessage(ctx context.Context, in *messages.BroadcastMessageRequest, opts ...client.CallOption) (*messages.BroadcastMessageResponse, error) {
	req := c.c.NewRequest(c.name, "MessagesService.BroadcastMessage", in)
	out := new(messages.BroadcastMessageResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *messagesService) BroadcastMessageNA(ctx context.Context, in *messages.BroadcastMessageRequest, opts ...client.CallOption) (*messages.BroadcastMessageResponse, error) {
	req := c.c.NewRequest(c.name, "MessagesService.BroadcastMessageNA", in)
	out := new(messages.BroadcastMessageResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for MessagesService service

type MessagesServiceHandler interface {
	// Sends a current user action event to a conversation partners.
	SendUserAction(context.Context, *SendUserActionRequest, *SendUserActionResponse) error
	// Broadcast message send message from via to peer recipients.
	BroadcastMessage(context.Context, *messages.BroadcastMessageRequest, *messages.BroadcastMessageResponse) error
	// Broadcast message send message from via to peer recipients (for internal services).
	BroadcastMessageNA(context.Context, *messages.BroadcastMessageRequest, *messages.BroadcastMessageResponse) error
}

func RegisterMessagesServiceHandler(s server.Server, hdlr MessagesServiceHandler, opts ...server.HandlerOption) error {
	type messagesService interface {
		SendUserAction(ctx context.Context, in *SendUserActionRequest, out *SendUserActionResponse) error
		BroadcastMessage(ctx context.Context, in *messages.BroadcastMessageRequest, out *messages.BroadcastMessageResponse) error
		BroadcastMessageNA(ctx context.Context, in *messages.BroadcastMessageRequest, out *messages.BroadcastMessageResponse) error
	}
	type MessagesService struct {
		messagesService
	}
	h := &messagesServiceHandler{hdlr}
	opts = append(opts, api.WithEndpoint(&api.Endpoint{
		Name:    "MessagesService.BroadcastMessage",
		Path:    []string{"/chat/broadcast"},
		Method:  []string{"POST"},
		Handler: "rpc",
	}))
	return s.Handle(s.NewHandler(&MessagesService{h}, opts...))
}

type messagesServiceHandler struct {
	MessagesServiceHandler
}

func (h *messagesServiceHandler) SendUserAction(ctx context.Context, in *SendUserActionRequest, out *SendUserActionResponse) error {
	return h.MessagesServiceHandler.SendUserAction(ctx, in, out)
}

func (h *messagesServiceHandler) BroadcastMessage(ctx context.Context, in *messages.BroadcastMessageRequest, out *messages.BroadcastMessageResponse) error {
	return h.MessagesServiceHandler.BroadcastMessage(ctx, in, out)
}

func (h *messagesServiceHandler) BroadcastMessageNA(ctx context.Context, in *messages.BroadcastMessageRequest, out *messages.BroadcastMessageResponse) error {
	return h.MessagesServiceHandler.BroadcastMessageNA(ctx, in, out)
}
