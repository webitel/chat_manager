package events

import (
	"github.com/webitel/chat_manager/api/proto/chat"
)

const (
	MessageEventType            = "message"
	MessageDeletedEventType     = "message_deleted"
	CloseConversationEventType  = "close_conversation"
	JoinConversationEventType   = "join_conversation"
	LeaveConversationEventType  = "leave_conversation"
	InviteConversationEventType = "invite_conversation"
	UserInvitationEventType     = "user_invite"
	DeclineInvitationEventType  = "decline_invite"
	UpdateChannelEventType      = "update_channel"
)

type BaseEvent struct {
	ConversationID string `json:"conversation_id"` // TO: channel.ID ! recepient !
	Timestamp      int64  `json:"timestamp"`
}

type MessageEvent struct {
	BaseEvent
	Message
}

type CloseConversationEvent struct {
	BaseEvent
	//FromUserID int64 `json:"from_user_id"`
	FromChannelID string `json:"from_channel_id"`
	Cause         string `json:"cause"`
}

type JoinConversationEvent struct {
	BaseEvent
	//JoinedUserID  int64 `json:"joined_user_id"`
	Member `json:"member"`
	//SelfChannelID string `json:"self_channel_id,omitempty"`
}

type LeaveConversationEvent struct {
	BaseEvent
	//LeavedUserID    int64  `json:"leaved_user_id"`
	LeavedChannelID string `json:"leaved_channel_id"`
	Cause           string `json:"cause,omitempty"`
}

type InviteConversationEvent struct {
	BaseEvent
	UserID int64 `json:"user_id"`
}

type UserInvitationEvent struct {
	BaseEvent
	Title        string            `json:"title"`
	InviteID     string            `json:"invite_id"`
	TimeoutSec   int64             `json:"timeout_sec"`
	Variables    map[string]string `json:"variables"`
	Conversation Conversation      `json:"conversation"`
	Members      []*Member         `json:"members"`
	Messages     []*Message        `json:"messages"`
}

type DeclineInvitationEvent struct {
	BaseEvent
	UserID   int64  `json:"user_id"`
	InviteID string `json:"invite_id"`
	Cause    string `json:"cause,omitempty"`
}

type UpdateChannelEvent struct {
	BaseEvent
	ChannelID string `json:"channel_id"`
	UpdatedAt int64  `json:"updated_at"`
}

type Conversation struct {
	ID        string `json:"id"`
	Title     string `json:"title,omitempty"`
	CreatedAt int64  `json:"created_at,omitempty"`
	//ClosedAt      int64     `json:"created_at,omitempty"`
	UpdatedAt int64 `json:"updated_at,omitempty"`
	//DomainID      int64     `json:"domain_id"`
	//SelfChannelID string    `json:"self_channel_id,omitempty"`
}

type Member struct {
	ChannelID  string `json:"id"`
	Type       string `json:"type"`
	Username   string `json:"name"`
	UserID     int64  `json:"user_id"`
	Internal   bool   `json:"internal"`
	ExternalId string `json:"external_id"`
	UpdatedAt  int64  `json:"updated_at,omitempty"`
	// Firstname string `json:"firstname,omitempty"`
	// Lastname  string `json:"lastname,omitempty"`
	Via *Gateway `json:"via,omitempty"`
}

type Gateway struct {
	Id   int64  `json:"id,omitempty"`
	Type string `json:"type,omitempty"`
	Name string `json:"name,omitempty"`
}

type Message struct {
	ID        int64  `json:"id"`                   // Unique Message.ID; TODO: within chat session !
	ChannelID string `json:"channel_id,omitempty"` // FROM: channel.ID ! sender !

	CreatedAt int64 `json:"created_at,omitempty"`
	UpdatedAt int64 `json:"updated_at,omitempty"` // EDITED !

	Type string `json:"type"` // "text" or "file"
	Text string `json:"text,omitempty"`
	File *File  `json:"file,omitempty"`

	Contact *Contact `json:"contact,omitempty"`
	// Reply Button Click[ed]
	Postback *Postback `json:"postback,omitempty"`

	ReplyToMessageID int64 `json:"reply_to_message_id,omitempty"`
	MessageForwarded       // embedded
}

// MessageForwarded event arguments
type MessageForwarded struct {
	//  ForwardFrom *User          `json:"forward_from,omitempty"`
	//  ForwardFromChat *Chat      `json:"forward_from_chat,omitempty"`
	ForwardFromChatID    string `json:"forward_from_chat_id,omitempty"`
	ForwardFromMessageID int64  `json:"forward_from_message_id,omitempty"`
	ForwardSenderName    string `json:"forward_sender_name,omitempty"`
	ForwardDate          int64  `json:"forward_date,omitempty"`
}

type File struct {
	ID   int64  `json:"id"`
	URL  string `json:"url,omitempty"`
	Type string `json:"mime"`
	Size int64  `json:"size"`
	Name string `json:"name"`
}

type Contact struct {
	ID        int64  `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Phone     string `json:"phone"`
}

type Postback = chat.Postback
