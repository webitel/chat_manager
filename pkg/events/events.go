package events

const (
	MessageEventType            = "message"
	CloseConversationEventType  = "close_conversation"
	JoinConversationEventType   = "join_conversation"
	LeaveConversationEventType  = "leave_conversation"
	InviteConversationEventType = "invite_conversation"
	UserInvitationEventType     = "user_invite"
	DeclineInvitationEventType  = "decline_invite"
	UpdateChannelEventType      = "update_channel"
)

type BaseEvent struct {
	ConversationID string `json:"conversation_id"` // FOXME: TO ?
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
	ChannelID string `json:"id"`
	UserID    int64  `json:"user_id"`
	Username  string `json:"name"`
	Type      string `json:"type"`
	Internal  bool   `json:"internal"`
	UpdatedAt int64  `json:"updated_at,omitempty"`
	// Firstname string `json:"firstname,omitempty"`
	// Lastname  string `json:"lastname,omitempty"`
}

type Message struct {
	ID        int64  `json:"id"`
	ChannelID string `json:"channel_id,omitempty"` // FIXME: TO ?
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	File	  *File  `json:"file,omitempty"`
	CreatedAt int64  `json:"created_at,omitempty"`
	UpdatedAt int64  `json:"updated_at,omitempty"`
}

type File struct {
	ID        int64  `json:"id"`
	Size      int64  `json:"size"`
	Mime      string `json:"mime"`
	Name      string `json:"name"`
}
