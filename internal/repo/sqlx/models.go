package sqlxrepo

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/jmoiron/sqlx/types"
)

var (
	channelAllColumns      = []string{"id", "type", "conversation_id", "user_id", "connection", "created_at", "internal", "closed_at", "updated_at", "domain_id", "flow_bridge", "name"}
	clientAllColumns       = []string{"id", "name", "number", "created_at", "activity_at", "external_id", "first_name", "last_name"}
	conversationAllColumns = []string{"id", "title", "created_at", "closed_at", "updated_at", "domain_id"}
	inviteAllColumns       = []string{"id", "conversation_id", "user_id", "title", "timeout_sec", "inviter_channel_id", "closed_at", "created_at", "domain_id"}
	messageAllColumns      = []string{"id", "channel_id", "conversation_id", "text", "created_at", "updated_at", "type"}
	profileAllColumns      = []string{"id", "name", "schema_id", "type", "variables", "domain_id"}
)

type Channel struct {
	ID             string         `db:"id" json:"id"`
	Type           string         `db:"type" json:"type"`
	ConversationID string         `db:"conversation_id" json:"conversation_id"`
	UserID         int64          `db:"user_id" json:"user_id"`
	Connection     sql.NullString `db:"connection" json:"connection,omitempty"`
	CreatedAt      time.Time      `db:"created_at" json:"created_at,omitempty"`
	Internal       bool           `db:"internal" json:"internal"`
	ClosedAt       sql.NullTime   `db:"closed_at" json:"closed_at,omitempty"`
	UpdatedAt      time.Time      `db:"updated_at" json:"updated_at,omitempty"`
	DomainID       int64          `db:"domain_id" json:"domain_id"`
	FlowBridge     bool           `db:"flow_bridge" json:"flow_bridge"`
	Name           string         `db:"name" json:"name"`
}

type Client struct {
	ID         int64          `db:"id" json:"id"`
	Name       sql.NullString `db:"name" json:"name,omitempty"`
	Number     sql.NullString `db:"number" json:"number,omitempty"`
	CreatedAt  time.Time      `db:"created_at" json:"created_at,omitempty"`
	ExternalID sql.NullString `db:"external_id" json:"external_id,omitempty"`
	FirstName  sql.NullString `db:"first_name" json:"first_name,omitempty"`
	LastName   sql.NullString `db:"last_name" json:"last_name,omitempty"`
}

type Conversation struct {
	ID            string         `db:"id" json:"id"`
	Title         sql.NullString `db:"title" json:"title,omitempty"`
	CreatedAt     time.Time      `db:"created_at" json:"created_at,omitempty"`
	ClosedAt      sql.NullTime   `db:"closed_at" json:"closed_at,omitempty"`
	UpdatedAt     time.Time      `db:"updated_at" json:"updated_at,omitempty"`
	DomainID      int64          `db:"domain_id" json:"domain_id"`
	Members       ConversationMembers
	Messages      ConversationMessages
	MembersBytes  []byte `db:"members" json:"members"`
	MessagesBytes []byte `db:"messages" json:"messages"`
}

type ConversationMembers []*ConversationMember

type ConversationMember struct {
	ID        string    `db:"id" json:"id"`
	Type      string    `db:"type" json:"type"`
	UserID    int64     `db:"user_id" json:"user_id"`
	CreatedAt time.Time `db:"created_at" json:"created_at,omitempty"`
	Internal  bool      `db:"internal" json:"internal"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at,omitempty"`
	Name      string    `db:"name" json:"name"`
}

func (c *ConversationMembers) Scan(src interface{}) error {
	return json.Unmarshal(src.([]byte), c)
}

func (c *ConversationMembers) Value() (driver.Value, error) {
	return json.Marshal(c)
}

type ConversationMessages []*ConversationMessage

type ConversationMessage struct {
	ID        int64  `db:"id" json:"id"`
	ChannelID string `db:"channel_id" json:"channel_id,omitempty"`
	//UserID    int64     `db:"user_id" json:"user_id,omitempty"`
	//UserType  string    `db:"user_type" json:"user_type,omitempty"`
	Text      string    `db:"text" json:"text,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at,omitempty"`
	Type      string    `db:"type" json:"type"`
}

func (c *ConversationMessages) Scan(src interface{}) error {
	return json.Unmarshal(src.([]byte), c)
}

func (c *ConversationMessages) Value() (driver.Value, error) {
	return json.Marshal(c)
}

type Invite struct {
	ID               string         `db:"id" json:"id"`
	ConversationID   string         `db:"conversation_id" json:"conversation_id"`
	UserID           int64          `db:"user_id" json:"user_id"`
	Title            sql.NullString `db:"title" json:"title,omitempty"`
	TimeoutSec       int64          `db:"timeout_sec" json:"timeout_sec"`
	InviterChannelID sql.NullString `db:"inviter_channel_id" json:"inviter_channel_id,omitempty"`
	ClosedAt         sql.NullTime   `db:"closed_at" json:"closed_at,omitempty"`
	CreatedAt        time.Time      `db:"created_at" json:"created_at,omitempty"`
	DomainID         int64          `db:"domain_id" json:"domain_id"`
}

type Message struct {
	ID             int64          `db:"id" json:"id"`
	ChannelID      sql.NullString `db:"channel_id" json:"channel_id,omitempty"`
	UserID         sql.NullInt64  `db:"user_id" json:"user_id,omitempty"`
	UserType       sql.NullString `db:"user_type" json:"user_type,omitempty"`
	ConversationID string         `db:"conversation_id" json:"conversation_id"`
	Text           sql.NullString `db:"text" json:"text,omitempty"`
	CreatedAt      time.Time      `db:"created_at" json:"created_at,omitempty"`
	UpdatedAt      time.Time      `db:"updated_at" json:"updated_at,omitempty"`
	Type           string         `db:"type" json:"type"`
}

type Profile struct {
	ID        int64          `db:"id" json:"id"`
	Name      string         `db:"name" json:"name"`
	SchemaID  sql.NullInt64  `db:"schema_id" json:"schema_id,omitempty"`
	Type      string         `db:"type" json:"type"`
	Variables types.JSONText `db:"variables" json:"variables"`
	DomainID  int64          `db:"domain_id" json:"domain_id"`
	CreatedAt time.Time      `db:"created_at" json:"created_at,omitempty"`
}

type WebitelUser struct {
	ID   int64  `db:"id" json:"id"`
	Name string `db:"name" json:"name"`
}
