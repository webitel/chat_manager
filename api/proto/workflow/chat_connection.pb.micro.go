// Code generated by protoc-gen-micro. DO NOT EDIT.
// source: chat_connection.proto

package workflow

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	_ "github.com/webitel/chat_manager/api/proto/chat"
	math "math"
)

import (
	context "context"
	api "github.com/micro/go-micro/v2/api"
	client "github.com/micro/go-micro/v2/client"
	server "github.com/micro/go-micro/v2/server"
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

// Api Endpoints for FlowChatServerService service

func NewFlowChatServerServiceEndpoints() []*api.Endpoint {
	return []*api.Endpoint{}
}

// Client API for FlowChatServerService service

type FlowChatServerService interface {
	Start(ctx context.Context, in *StartRequest, opts ...client.CallOption) (*StartResponse, error)
	Break(ctx context.Context, in *BreakRequest, opts ...client.CallOption) (*BreakResponse, error)
	BreakBridge(ctx context.Context, in *BreakBridgeRequest, opts ...client.CallOption) (*BreakBridgeResponse, error)
	ConfirmationMessage(ctx context.Context, in *ConfirmationMessageRequest, opts ...client.CallOption) (*ConfirmationMessageResponse, error)
	TransferChatPlan(ctx context.Context, in *TransferChatPlanRequest, opts ...client.CallOption) (*TransferChatPlanResponse, error)
}

type flowChatServerService struct {
	c    client.Client
	name string
}

func NewFlowChatServerService(name string, c client.Client) FlowChatServerService {
	return &flowChatServerService{
		c:    c,
		name: name,
	}
}

func (c *flowChatServerService) Start(ctx context.Context, in *StartRequest, opts ...client.CallOption) (*StartResponse, error) {
	req := c.c.NewRequest(c.name, "FlowChatServerService.Start", in)
	out := new(StartResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *flowChatServerService) Break(ctx context.Context, in *BreakRequest, opts ...client.CallOption) (*BreakResponse, error) {
	req := c.c.NewRequest(c.name, "FlowChatServerService.Break", in)
	out := new(BreakResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *flowChatServerService) BreakBridge(ctx context.Context, in *BreakBridgeRequest, opts ...client.CallOption) (*BreakBridgeResponse, error) {
	req := c.c.NewRequest(c.name, "FlowChatServerService.BreakBridge", in)
	out := new(BreakBridgeResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *flowChatServerService) ConfirmationMessage(ctx context.Context, in *ConfirmationMessageRequest, opts ...client.CallOption) (*ConfirmationMessageResponse, error) {
	req := c.c.NewRequest(c.name, "FlowChatServerService.ConfirmationMessage", in)
	out := new(ConfirmationMessageResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *flowChatServerService) TransferChatPlan(ctx context.Context, in *TransferChatPlanRequest, opts ...client.CallOption) (*TransferChatPlanResponse, error) {
	req := c.c.NewRequest(c.name, "FlowChatServerService.TransferChatPlan", in)
	out := new(TransferChatPlanResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for FlowChatServerService service

type FlowChatServerServiceHandler interface {
	Start(context.Context, *StartRequest, *StartResponse) error
	Break(context.Context, *BreakRequest, *BreakResponse) error
	BreakBridge(context.Context, *BreakBridgeRequest, *BreakBridgeResponse) error
	ConfirmationMessage(context.Context, *ConfirmationMessageRequest, *ConfirmationMessageResponse) error
	TransferChatPlan(context.Context, *TransferChatPlanRequest, *TransferChatPlanResponse) error
}

func RegisterFlowChatServerServiceHandler(s server.Server, hdlr FlowChatServerServiceHandler, opts ...server.HandlerOption) error {
	type flowChatServerService interface {
		Start(ctx context.Context, in *StartRequest, out *StartResponse) error
		Break(ctx context.Context, in *BreakRequest, out *BreakResponse) error
		BreakBridge(ctx context.Context, in *BreakBridgeRequest, out *BreakBridgeResponse) error
		ConfirmationMessage(ctx context.Context, in *ConfirmationMessageRequest, out *ConfirmationMessageResponse) error
		TransferChatPlan(ctx context.Context, in *TransferChatPlanRequest, out *TransferChatPlanResponse) error
	}
	type FlowChatServerService struct {
		flowChatServerService
	}
	h := &flowChatServerServiceHandler{hdlr}
	return s.Handle(s.NewHandler(&FlowChatServerService{h}, opts...))
}

type flowChatServerServiceHandler struct {
	FlowChatServerServiceHandler
}

func (h *flowChatServerServiceHandler) Start(ctx context.Context, in *StartRequest, out *StartResponse) error {
	return h.FlowChatServerServiceHandler.Start(ctx, in, out)
}

func (h *flowChatServerServiceHandler) Break(ctx context.Context, in *BreakRequest, out *BreakResponse) error {
	return h.FlowChatServerServiceHandler.Break(ctx, in, out)
}

func (h *flowChatServerServiceHandler) BreakBridge(ctx context.Context, in *BreakBridgeRequest, out *BreakBridgeResponse) error {
	return h.FlowChatServerServiceHandler.BreakBridge(ctx, in, out)
}

func (h *flowChatServerServiceHandler) ConfirmationMessage(ctx context.Context, in *ConfirmationMessageRequest, out *ConfirmationMessageResponse) error {
	return h.FlowChatServerServiceHandler.ConfirmationMessage(ctx, in, out)
}

func (h *flowChatServerServiceHandler) TransferChatPlan(ctx context.Context, in *TransferChatPlanRequest, out *TransferChatPlanResponse) error {
	return h.FlowChatServerServiceHandler.TransferChatPlan(ctx, in, out)
}
