package events

const (
	MessageEventType            = "message"
	CloseConversationEventType  = "close_conversation"
	JoinConversationEventType   = "join_conversation"
	LeaveConversationEventType  = "leave_conversation"
	InviteConversationEventType = "invite_conversation"
	UserInvitationEventType     = "user_invite"
	DeclineInvitationEventType  = "decline_invite"
)

type BaseEvent struct {
	ConversationID string `json:"conversation_id"`
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
}

type InviteConversationEvent struct {
	BaseEvent
	UserID int64 `json:"user_id"`
}

type UserInvitationEvent struct {
	BaseEvent
	InviteID     string `json:"invite_id"`
	Title        string `json:"title"`
	Conversation `json:"conversation"`
	Members      []*Member  `json:"members"`
	Messages     []*Message `json:"messages"`
}

type DeclineInvitationEvent struct {
	BaseEvent
	UserID   int64  `json:"user_id"`
	InviteID string `json:"invite_id"`
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
	ChannelID string `json:"channel_id"`
	UserID    int64  `json:"user_id"`
	Username  string `json:"username"`
	Type      string `json:"type"`
	Internal  bool   `json:"internal"`
	UpdatedAt int64  `json:"updated_at,omitempty"`
	// Firstname string `json:"firstname,omitempty"`
	// Lastname  string `json:"lastname,omitempty"`
}

type Message struct {
	ChannelID string `json:"channel_id,omitempty"`
	MessageID int64  `json:"message_id"`
	Type      string `json:"message_type"`
	Value     string `json:"message_value"`
	CreatedAt int64  `json:"created_at,omitempty"`
	UpdatedAt int64  `json:"updated_at,omitempty"`
}
