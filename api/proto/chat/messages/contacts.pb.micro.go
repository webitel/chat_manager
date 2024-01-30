// Code generated by protoc-gen-micro. DO NOT EDIT.
// source: chat/messages/contacts.proto

package messages

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/options"
	_ "google.golang.org/genproto/googleapis/api/annotations"
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

// Api Endpoints for ContactLinkingService service

func NewContactLinkingServiceEndpoints() []*api.Endpoint {
	return []*api.Endpoint{
		&api.Endpoint{
			Name:    "ContactLinkingService.LinkContactToClient",
			Path:    []string{"/chat/{conversation_id}/link"},
			Method:  []string{"POST"},
			Body:    "",
			Handler: "rpc",
		},
		&api.Endpoint{
			Name:    "ContactLinkingService.CreateContactFromConversation",
			Path:    []string{"/chat/{conversation_id}/contact"},
			Method:  []string{"POST"},
			Body:    "",
			Handler: "rpc",
		},
	}
}

// Client API for ContactLinkingService service

type ContactLinkingService interface {
	// Query of the chat history
	LinkContactToClient(ctx context.Context, in *LinkContactToClientRequest, opts ...client.CallOption) (*EmptyResponse, error)
	// Query of the chat history
	CreateContactFromConversation(ctx context.Context, in *CreateContactFromConversationRequest, opts ...client.CallOption) (*EmptyResponse, error)
}

type contactLinkingService struct {
	c    client.Client
	name string
}

func NewContactLinkingService(name string, c client.Client) ContactLinkingService {
	return &contactLinkingService{
		c:    c,
		name: name,
	}
}

func (c *contactLinkingService) LinkContactToClient(ctx context.Context, in *LinkContactToClientRequest, opts ...client.CallOption) (*EmptyResponse, error) {
	req := c.c.NewRequest(c.name, "ContactLinkingService.LinkContactToClient", in)
	out := new(EmptyResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *contactLinkingService) CreateContactFromConversation(ctx context.Context, in *CreateContactFromConversationRequest, opts ...client.CallOption) (*EmptyResponse, error) {
	req := c.c.NewRequest(c.name, "ContactLinkingService.CreateContactFromConversation", in)
	out := new(EmptyResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for ContactLinkingService service

type ContactLinkingServiceHandler interface {
	// Query of the chat history
	LinkContactToClient(context.Context, *LinkContactToClientRequest, *EmptyResponse) error
	// Query of the chat history
	CreateContactFromConversation(context.Context, *CreateContactFromConversationRequest, *EmptyResponse) error
}

func RegisterContactLinkingServiceHandler(s server.Server, hdlr ContactLinkingServiceHandler, opts ...server.HandlerOption) error {
	type contactLinkingService interface {
		LinkContactToClient(ctx context.Context, in *LinkContactToClientRequest, out *EmptyResponse) error
		CreateContactFromConversation(ctx context.Context, in *CreateContactFromConversationRequest, out *EmptyResponse) error
	}
	type ContactLinkingService struct {
		contactLinkingService
	}
	h := &contactLinkingServiceHandler{hdlr}
	opts = append(opts, api.WithEndpoint(&api.Endpoint{
		Name:    "ContactLinkingService.LinkContactToClient",
		Path:    []string{"/chat/{conversation_id}/link"},
		Method:  []string{"POST"},
		Body:    "",
		Handler: "rpc",
	}))
	opts = append(opts, api.WithEndpoint(&api.Endpoint{
		Name:    "ContactLinkingService.CreateContactFromConversation",
		Path:    []string{"/chat/{conversation_id}/contact"},
		Method:  []string{"POST"},
		Body:    "",
		Handler: "rpc",
	}))
	return s.Handle(s.NewHandler(&ContactLinkingService{h}, opts...))
}

type contactLinkingServiceHandler struct {
	ContactLinkingServiceHandler
}

func (h *contactLinkingServiceHandler) LinkContactToClient(ctx context.Context, in *LinkContactToClientRequest, out *EmptyResponse) error {
	return h.ContactLinkingServiceHandler.LinkContactToClient(ctx, in, out)
}

func (h *contactLinkingServiceHandler) CreateContactFromConversation(ctx context.Context, in *CreateContactFromConversationRequest, out *EmptyResponse) error {
	return h.ContactLinkingServiceHandler.CreateContactFromConversation(ctx, in, out)
}
