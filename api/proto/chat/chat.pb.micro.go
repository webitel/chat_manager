// Code generated by protoc-gen-micro. DO NOT EDIT.
// source: chat.proto

package chat

import (
	fmt "fmt"
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

// Api Endpoints for ChatService service

func NewChatServiceEndpoints() []*api.Endpoint {
	return []*api.Endpoint{}
}

// Client API for ChatService service

type ChatService interface {
	// SendMessage [FROM] created channel_id (+auth_user_id) [TO] conversation_id chat-room
	SendMessage(ctx context.Context, in *SendMessageRequest, opts ...client.CallOption) (*SendMessageResponse, error)
	// StartConversation starts bot's (.user.type:.user.connection) flow schema NEW routine
	StartConversation(ctx context.Context, in *StartConversationRequest, opts ...client.CallOption) (*StartConversationResponse, error)
	// CloseConversation stops and close chat-bot's schema routine with all it's recipient(s)
	CloseConversation(ctx context.Context, in *CloseConversationRequest, opts ...client.CallOption) (*CloseConversationResponse, error)
	// JoinConversation accepts user's invitation to chat conversation
	JoinConversation(ctx context.Context, in *JoinConversationRequest, opts ...client.CallOption) (*JoinConversationResponse, error)
	// LeaveConversation kicks requested user from chat conversation
	LeaveConversation(ctx context.Context, in *LeaveConversationRequest, opts ...client.CallOption) (*LeaveConversationResponse, error)
	// InviteToConversation publish NEW invitation for .user
	InviteToConversation(ctx context.Context, in *InviteToConversationRequest, opts ...client.CallOption) (*InviteToConversationResponse, error)
	// DeclineInvitation declines chat invitation FROM user
	DeclineInvitation(ctx context.Context, in *DeclineInvitationRequest, opts ...client.CallOption) (*DeclineInvitationResponse, error)
	// DeleteMessage by unique `id` or `variables` as external binding(s)
	DeleteMessage(ctx context.Context, in *DeleteMessageRequest, opts ...client.CallOption) (*HistoryMessage, error)
	// CheckSession returns internal chat channel for external chat user
	CheckSession(ctx context.Context, in *CheckSessionRequest, opts ...client.CallOption) (*CheckSessionResponse, error)
	WaitMessage(ctx context.Context, in *WaitMessageRequest, opts ...client.CallOption) (*WaitMessageResponse, error)
	UpdateChannel(ctx context.Context, in *UpdateChannelRequest, opts ...client.CallOption) (*UpdateChannelResponse, error)
	GetChannelByPeer(ctx context.Context, in *GetChannelByPeerRequest, opts ...client.CallOption) (*Channel, error)
	GetConversations(ctx context.Context, in *GetConversationsRequest, opts ...client.CallOption) (*GetConversationsResponse, error)
	GetConversationByID(ctx context.Context, in *GetConversationByIDRequest, opts ...client.CallOption) (*GetConversationByIDResponse, error)
	GetHistoryMessages(ctx context.Context, in *GetHistoryMessagesRequest, opts ...client.CallOption) (*GetHistoryMessagesResponse, error)
	// [WTEL-4695] crutch
	SaveAgentJoinMessage(ctx context.Context, in *SaveAgentJoinMessageRequest, opts ...client.CallOption) (*SaveAgentJoinMessageResponse, error)
	// API /v1
	SetVariables(ctx context.Context, in *SetVariablesRequest, opts ...client.CallOption) (*ChatVariablesResponse, error)
	BlindTransfer(ctx context.Context, in *ChatTransferRequest, opts ...client.CallOption) (*ChatTransferResponse, error)
}

type chatService struct {
	c    client.Client
	name string
}

func NewChatService(name string, c client.Client) ChatService {
	return &chatService{
		c:    c,
		name: name,
	}
}

func (c *chatService) SendMessage(ctx context.Context, in *SendMessageRequest, opts ...client.CallOption) (*SendMessageResponse, error) {
	req := c.c.NewRequest(c.name, "ChatService.SendMessage", in)
	out := new(SendMessageResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *chatService) StartConversation(ctx context.Context, in *StartConversationRequest, opts ...client.CallOption) (*StartConversationResponse, error) {
	req := c.c.NewRequest(c.name, "ChatService.StartConversation", in)
	out := new(StartConversationResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *chatService) CloseConversation(ctx context.Context, in *CloseConversationRequest, opts ...client.CallOption) (*CloseConversationResponse, error) {
	req := c.c.NewRequest(c.name, "ChatService.CloseConversation", in)
	out := new(CloseConversationResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *chatService) JoinConversation(ctx context.Context, in *JoinConversationRequest, opts ...client.CallOption) (*JoinConversationResponse, error) {
	req := c.c.NewRequest(c.name, "ChatService.JoinConversation", in)
	out := new(JoinConversationResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *chatService) LeaveConversation(ctx context.Context, in *LeaveConversationRequest, opts ...client.CallOption) (*LeaveConversationResponse, error) {
	req := c.c.NewRequest(c.name, "ChatService.LeaveConversation", in)
	out := new(LeaveConversationResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *chatService) InviteToConversation(ctx context.Context, in *InviteToConversationRequest, opts ...client.CallOption) (*InviteToConversationResponse, error) {
	req := c.c.NewRequest(c.name, "ChatService.InviteToConversation", in)
	out := new(InviteToConversationResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *chatService) DeclineInvitation(ctx context.Context, in *DeclineInvitationRequest, opts ...client.CallOption) (*DeclineInvitationResponse, error) {
	req := c.c.NewRequest(c.name, "ChatService.DeclineInvitation", in)
	out := new(DeclineInvitationResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *chatService) DeleteMessage(ctx context.Context, in *DeleteMessageRequest, opts ...client.CallOption) (*HistoryMessage, error) {
	req := c.c.NewRequest(c.name, "ChatService.DeleteMessage", in)
	out := new(HistoryMessage)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *chatService) CheckSession(ctx context.Context, in *CheckSessionRequest, opts ...client.CallOption) (*CheckSessionResponse, error) {
	req := c.c.NewRequest(c.name, "ChatService.CheckSession", in)
	out := new(CheckSessionResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *chatService) WaitMessage(ctx context.Context, in *WaitMessageRequest, opts ...client.CallOption) (*WaitMessageResponse, error) {
	req := c.c.NewRequest(c.name, "ChatService.WaitMessage", in)
	out := new(WaitMessageResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *chatService) UpdateChannel(ctx context.Context, in *UpdateChannelRequest, opts ...client.CallOption) (*UpdateChannelResponse, error) {
	req := c.c.NewRequest(c.name, "ChatService.UpdateChannel", in)
	out := new(UpdateChannelResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *chatService) GetChannelByPeer(ctx context.Context, in *GetChannelByPeerRequest, opts ...client.CallOption) (*Channel, error) {
	req := c.c.NewRequest(c.name, "ChatService.GetChannelByPeer", in)
	out := new(Channel)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *chatService) GetConversations(ctx context.Context, in *GetConversationsRequest, opts ...client.CallOption) (*GetConversationsResponse, error) {
	req := c.c.NewRequest(c.name, "ChatService.GetConversations", in)
	out := new(GetConversationsResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *chatService) GetConversationByID(ctx context.Context, in *GetConversationByIDRequest, opts ...client.CallOption) (*GetConversationByIDResponse, error) {
	req := c.c.NewRequest(c.name, "ChatService.GetConversationByID", in)
	out := new(GetConversationByIDResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *chatService) GetHistoryMessages(ctx context.Context, in *GetHistoryMessagesRequest, opts ...client.CallOption) (*GetHistoryMessagesResponse, error) {
	req := c.c.NewRequest(c.name, "ChatService.GetHistoryMessages", in)
	out := new(GetHistoryMessagesResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *chatService) SaveAgentJoinMessage(ctx context.Context, in *SaveAgentJoinMessageRequest, opts ...client.CallOption) (*SaveAgentJoinMessageResponse, error) {
	req := c.c.NewRequest(c.name, "ChatService.SaveAgentJoinMessage", in)
	out := new(SaveAgentJoinMessageResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *chatService) SetVariables(ctx context.Context, in *SetVariablesRequest, opts ...client.CallOption) (*ChatVariablesResponse, error) {
	req := c.c.NewRequest(c.name, "ChatService.SetVariables", in)
	out := new(ChatVariablesResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *chatService) BlindTransfer(ctx context.Context, in *ChatTransferRequest, opts ...client.CallOption) (*ChatTransferResponse, error) {
	req := c.c.NewRequest(c.name, "ChatService.BlindTransfer", in)
	out := new(ChatTransferResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for ChatService service

type ChatServiceHandler interface {
	// SendMessage [FROM] created channel_id (+auth_user_id) [TO] conversation_id chat-room
	SendMessage(context.Context, *SendMessageRequest, *SendMessageResponse) error
	// StartConversation starts bot's (.user.type:.user.connection) flow schema NEW routine
	StartConversation(context.Context, *StartConversationRequest, *StartConversationResponse) error
	// CloseConversation stops and close chat-bot's schema routine with all it's recipient(s)
	CloseConversation(context.Context, *CloseConversationRequest, *CloseConversationResponse) error
	// JoinConversation accepts user's invitation to chat conversation
	JoinConversation(context.Context, *JoinConversationRequest, *JoinConversationResponse) error
	// LeaveConversation kicks requested user from chat conversation
	LeaveConversation(context.Context, *LeaveConversationRequest, *LeaveConversationResponse) error
	// InviteToConversation publish NEW invitation for .user
	InviteToConversation(context.Context, *InviteToConversationRequest, *InviteToConversationResponse) error
	// DeclineInvitation declines chat invitation FROM user
	DeclineInvitation(context.Context, *DeclineInvitationRequest, *DeclineInvitationResponse) error
	// DeleteMessage by unique `id` or `variables` as external binding(s)
	DeleteMessage(context.Context, *DeleteMessageRequest, *HistoryMessage) error
	// CheckSession returns internal chat channel for external chat user
	CheckSession(context.Context, *CheckSessionRequest, *CheckSessionResponse) error
	WaitMessage(context.Context, *WaitMessageRequest, *WaitMessageResponse) error
	UpdateChannel(context.Context, *UpdateChannelRequest, *UpdateChannelResponse) error
	GetChannelByPeer(context.Context, *GetChannelByPeerRequest, *Channel) error
	GetConversations(context.Context, *GetConversationsRequest, *GetConversationsResponse) error
	GetConversationByID(context.Context, *GetConversationByIDRequest, *GetConversationByIDResponse) error
	GetHistoryMessages(context.Context, *GetHistoryMessagesRequest, *GetHistoryMessagesResponse) error
	// [WTEL-4695] crutch
	SaveAgentJoinMessage(context.Context, *SaveAgentJoinMessageRequest, *SaveAgentJoinMessageResponse) error
	// API /v1
	SetVariables(context.Context, *SetVariablesRequest, *ChatVariablesResponse) error
	BlindTransfer(context.Context, *ChatTransferRequest, *ChatTransferResponse) error
}

func RegisterChatServiceHandler(s server.Server, hdlr ChatServiceHandler, opts ...server.HandlerOption) error {
	type chatService interface {
		SendMessage(ctx context.Context, in *SendMessageRequest, out *SendMessageResponse) error
		StartConversation(ctx context.Context, in *StartConversationRequest, out *StartConversationResponse) error
		CloseConversation(ctx context.Context, in *CloseConversationRequest, out *CloseConversationResponse) error
		JoinConversation(ctx context.Context, in *JoinConversationRequest, out *JoinConversationResponse) error
		LeaveConversation(ctx context.Context, in *LeaveConversationRequest, out *LeaveConversationResponse) error
		InviteToConversation(ctx context.Context, in *InviteToConversationRequest, out *InviteToConversationResponse) error
		DeclineInvitation(ctx context.Context, in *DeclineInvitationRequest, out *DeclineInvitationResponse) error
		DeleteMessage(ctx context.Context, in *DeleteMessageRequest, out *HistoryMessage) error
		CheckSession(ctx context.Context, in *CheckSessionRequest, out *CheckSessionResponse) error
		WaitMessage(ctx context.Context, in *WaitMessageRequest, out *WaitMessageResponse) error
		UpdateChannel(ctx context.Context, in *UpdateChannelRequest, out *UpdateChannelResponse) error
		GetChannelByPeer(ctx context.Context, in *GetChannelByPeerRequest, out *Channel) error
		GetConversations(ctx context.Context, in *GetConversationsRequest, out *GetConversationsResponse) error
		GetConversationByID(ctx context.Context, in *GetConversationByIDRequest, out *GetConversationByIDResponse) error
		GetHistoryMessages(ctx context.Context, in *GetHistoryMessagesRequest, out *GetHistoryMessagesResponse) error
		SaveAgentJoinMessage(ctx context.Context, in *SaveAgentJoinMessageRequest, out *SaveAgentJoinMessageResponse) error
		SetVariables(ctx context.Context, in *SetVariablesRequest, out *ChatVariablesResponse) error
		BlindTransfer(ctx context.Context, in *ChatTransferRequest, out *ChatTransferResponse) error
	}
	type ChatService struct {
		chatService
	}
	h := &chatServiceHandler{hdlr}
	return s.Handle(s.NewHandler(&ChatService{h}, opts...))
}

type chatServiceHandler struct {
	ChatServiceHandler
}

func (h *chatServiceHandler) SendMessage(ctx context.Context, in *SendMessageRequest, out *SendMessageResponse) error {
	return h.ChatServiceHandler.SendMessage(ctx, in, out)
}

func (h *chatServiceHandler) StartConversation(ctx context.Context, in *StartConversationRequest, out *StartConversationResponse) error {
	return h.ChatServiceHandler.StartConversation(ctx, in, out)
}

func (h *chatServiceHandler) CloseConversation(ctx context.Context, in *CloseConversationRequest, out *CloseConversationResponse) error {
	return h.ChatServiceHandler.CloseConversation(ctx, in, out)
}

func (h *chatServiceHandler) JoinConversation(ctx context.Context, in *JoinConversationRequest, out *JoinConversationResponse) error {
	return h.ChatServiceHandler.JoinConversation(ctx, in, out)
}

func (h *chatServiceHandler) LeaveConversation(ctx context.Context, in *LeaveConversationRequest, out *LeaveConversationResponse) error {
	return h.ChatServiceHandler.LeaveConversation(ctx, in, out)
}

func (h *chatServiceHandler) InviteToConversation(ctx context.Context, in *InviteToConversationRequest, out *InviteToConversationResponse) error {
	return h.ChatServiceHandler.InviteToConversation(ctx, in, out)
}

func (h *chatServiceHandler) DeclineInvitation(ctx context.Context, in *DeclineInvitationRequest, out *DeclineInvitationResponse) error {
	return h.ChatServiceHandler.DeclineInvitation(ctx, in, out)
}

func (h *chatServiceHandler) DeleteMessage(ctx context.Context, in *DeleteMessageRequest, out *HistoryMessage) error {
	return h.ChatServiceHandler.DeleteMessage(ctx, in, out)
}

func (h *chatServiceHandler) CheckSession(ctx context.Context, in *CheckSessionRequest, out *CheckSessionResponse) error {
	return h.ChatServiceHandler.CheckSession(ctx, in, out)
}

func (h *chatServiceHandler) WaitMessage(ctx context.Context, in *WaitMessageRequest, out *WaitMessageResponse) error {
	return h.ChatServiceHandler.WaitMessage(ctx, in, out)
}

func (h *chatServiceHandler) UpdateChannel(ctx context.Context, in *UpdateChannelRequest, out *UpdateChannelResponse) error {
	return h.ChatServiceHandler.UpdateChannel(ctx, in, out)
}

func (h *chatServiceHandler) GetChannelByPeer(ctx context.Context, in *GetChannelByPeerRequest, out *Channel) error {
	return h.ChatServiceHandler.GetChannelByPeer(ctx, in, out)
}

func (h *chatServiceHandler) GetConversations(ctx context.Context, in *GetConversationsRequest, out *GetConversationsResponse) error {
	return h.ChatServiceHandler.GetConversations(ctx, in, out)
}

func (h *chatServiceHandler) GetConversationByID(ctx context.Context, in *GetConversationByIDRequest, out *GetConversationByIDResponse) error {
	return h.ChatServiceHandler.GetConversationByID(ctx, in, out)
}

func (h *chatServiceHandler) GetHistoryMessages(ctx context.Context, in *GetHistoryMessagesRequest, out *GetHistoryMessagesResponse) error {
	return h.ChatServiceHandler.GetHistoryMessages(ctx, in, out)
}

func (h *chatServiceHandler) SaveAgentJoinMessage(ctx context.Context, in *SaveAgentJoinMessageRequest, out *SaveAgentJoinMessageResponse) error {
	return h.ChatServiceHandler.SaveAgentJoinMessage(ctx, in, out)
}

func (h *chatServiceHandler) SetVariables(ctx context.Context, in *SetVariablesRequest, out *ChatVariablesResponse) error {
	return h.ChatServiceHandler.SetVariables(ctx, in, out)
}

func (h *chatServiceHandler) BlindTransfer(ctx context.Context, in *ChatTransferRequest, out *ChatTransferResponse) error {
	return h.ChatServiceHandler.BlindTransfer(ctx, in, out)
}
