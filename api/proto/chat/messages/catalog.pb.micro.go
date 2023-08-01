// Code generated by protoc-gen-micro. DO NOT EDIT.
// source: chat/messages/catalog.proto

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

// Api Endpoints for Catalog service

func NewCatalogEndpoints() []*api.Endpoint {
	return []*api.Endpoint{
		&api.Endpoint{
			Name:    "Catalog.GetCustomers",
			Path:    []string{"/chat/customers"},
			Method:  []string{"GET"},
			Handler: "rpc",
		},
		&api.Endpoint{
			Name:    "Catalog.GetDialogs",
			Path:    []string{"/chat/dialogs"},
			Method:  []string{"GET"},
			Handler: "rpc",
		},
		&api.Endpoint{
			Name:    "Catalog.GetMembers",
			Path:    []string{"/chat/dialogs/{chat_id}/members"},
			Method:  []string{"GET"},
			Handler: "rpc",
		},
		&api.Endpoint{
			Name:    "Catalog.GetHistory",
			Path:    []string{"/chat/dialogs/{chat_id}/messages"},
			Method:  []string{"GET"},
			Handler: "rpc",
		},
	}
}

// Client API for Catalog service

type CatalogService interface {
	// Query of external chat customers
	GetCustomers(ctx context.Context, in *ChatCustomersRequest, opts ...client.CallOption) (*ChatCustomers, error)
	// Query of chat conversations
	GetDialogs(ctx context.Context, in *ChatDialogsRequest, opts ...client.CallOption) (*ChatDialogs, error)
	// Query of chat participants
	GetMembers(ctx context.Context, in *ChatMembersRequest, opts ...client.CallOption) (*ChatMembers, error)
	// Query of chat messages history
	GetHistory(ctx context.Context, in *ChatMessagesRequest, opts ...client.CallOption) (*ChatMessages, error)
}

type catalogService struct {
	c    client.Client
	name string
}

func NewCatalogService(name string, c client.Client) CatalogService {
	return &catalogService{
		c:    c,
		name: name,
	}
}

func (c *catalogService) GetCustomers(ctx context.Context, in *ChatCustomersRequest, opts ...client.CallOption) (*ChatCustomers, error) {
	req := c.c.NewRequest(c.name, "Catalog.GetCustomers", in)
	out := new(ChatCustomers)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *catalogService) GetDialogs(ctx context.Context, in *ChatDialogsRequest, opts ...client.CallOption) (*ChatDialogs, error) {
	req := c.c.NewRequest(c.name, "Catalog.GetDialogs", in)
	out := new(ChatDialogs)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *catalogService) GetMembers(ctx context.Context, in *ChatMembersRequest, opts ...client.CallOption) (*ChatMembers, error) {
	req := c.c.NewRequest(c.name, "Catalog.GetMembers", in)
	out := new(ChatMembers)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *catalogService) GetHistory(ctx context.Context, in *ChatMessagesRequest, opts ...client.CallOption) (*ChatMessages, error) {
	req := c.c.NewRequest(c.name, "Catalog.GetHistory", in)
	out := new(ChatMessages)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for Catalog service

type CatalogHandler interface {
	// Query of external chat customers
	GetCustomers(context.Context, *ChatCustomersRequest, *ChatCustomers) error
	// Query of chat conversations
	GetDialogs(context.Context, *ChatDialogsRequest, *ChatDialogs) error
	// Query of chat participants
	GetMembers(context.Context, *ChatMembersRequest, *ChatMembers) error
	// Query of chat messages history
	GetHistory(context.Context, *ChatMessagesRequest, *ChatMessages) error
}

func RegisterCatalogHandler(s server.Server, hdlr CatalogHandler, opts ...server.HandlerOption) error {
	type catalog interface {
		GetCustomers(ctx context.Context, in *ChatCustomersRequest, out *ChatCustomers) error
		GetDialogs(ctx context.Context, in *ChatDialogsRequest, out *ChatDialogs) error
		GetMembers(ctx context.Context, in *ChatMembersRequest, out *ChatMembers) error
		GetHistory(ctx context.Context, in *ChatMessagesRequest, out *ChatMessages) error
	}
	type Catalog struct {
		catalog
	}
	h := &catalogHandler{hdlr}
	opts = append(opts, api.WithEndpoint(&api.Endpoint{
		Name:    "Catalog.GetCustomers",
		Path:    []string{"/chat/customers"},
		Method:  []string{"GET"},
		Handler: "rpc",
	}))
	opts = append(opts, api.WithEndpoint(&api.Endpoint{
		Name:    "Catalog.GetDialogs",
		Path:    []string{"/chat/dialogs"},
		Method:  []string{"GET"},
		Handler: "rpc",
	}))
	opts = append(opts, api.WithEndpoint(&api.Endpoint{
		Name:    "Catalog.GetMembers",
		Path:    []string{"/chat/dialogs/{chat_id}/members"},
		Method:  []string{"GET"},
		Handler: "rpc",
	}))
	opts = append(opts, api.WithEndpoint(&api.Endpoint{
		Name:    "Catalog.GetHistory",
		Path:    []string{"/chat/dialogs/{chat_id}/messages"},
		Method:  []string{"GET"},
		Handler: "rpc",
	}))
	return s.Handle(s.NewHandler(&Catalog{h}, opts...))
}

type catalogHandler struct {
	CatalogHandler
}

func (h *catalogHandler) GetCustomers(ctx context.Context, in *ChatCustomersRequest, out *ChatCustomers) error {
	return h.CatalogHandler.GetCustomers(ctx, in, out)
}

func (h *catalogHandler) GetDialogs(ctx context.Context, in *ChatDialogsRequest, out *ChatDialogs) error {
	return h.CatalogHandler.GetDialogs(ctx, in, out)
}

func (h *catalogHandler) GetMembers(ctx context.Context, in *ChatMembersRequest, out *ChatMembers) error {
	return h.CatalogHandler.GetMembers(ctx, in, out)
}

func (h *catalogHandler) GetHistory(ctx context.Context, in *ChatMessagesRequest, out *ChatMessages) error {
	return h.CatalogHandler.GetHistory(ctx, in, out)
}
