package sqlxrepo

import (
	"database/sql"

	"github.com/jmoiron/sqlx/types"
)

type Channel struct {
	ID             string         `sql:"id" json:"id"`
	Type           string         `sql:"type" json:"type"`
	ConversationID string         `sql:"conversation_id" json:"conversation_id"`
	UserID         int64          `sql:"user_id" json:"user_id"`
	Connection     sql.NullString `sql:"connection" json:"connection,omitempty"`
	CreatedAt      sql.NullTime   `sql:"created_at" json:"created_at,omitempty"`
	Internal       bool           `sql:"internal" json:"internal"`
	ClosedAt       sql.NullTime   `sql:"closed_at" json:"closed_at,omitempty"`
	DomainID       int64          `sql:"domain_id" json:"domain_id"`
	FlowBridge     bool           `sql:"flow_bridge" json:"flow_bridge"`
}

type Client struct {
	ID         int64          `sql:"id" json:"id"`
	Name       sql.NullString `sql:"name" json:"name,omitempty"`
	Number     sql.NullString `sql:"number" json:"number,omitempty"`
	CreatedAt  sql.NullTime   `sql:"created_at" json:"created_at,omitempty"`
	ActivityAt sql.NullTime   `sql:"activity_at" json:"activity_at,omitempty"`
	ExternalID sql.NullString `sql:"external_id" json:"external_id,omitempty"`
	FirstName  sql.NullString `sql:"first_name" json:"first_name,omitempty"`
	LastName   sql.NullString `sql:"last_name" json:"last_name,omitempty"`
}

type Conversation struct {
	ID        string         `sql:"id" json:"id"`
	Title     sql.NullString `sql:"title" json:"title,omitempty"`
	CreatedAt sql.NullTime   `sql:"created_at" json:"created_at,omitempty"`
	ClosedAt  sql.NullTime   `sql:"closed_at" json:"closed_at,omitempty"`
	UpdatedAt sql.NullTime   `sql:"updated_at" json:"updated_at,omitempty"`
	DomainID  int64          `sql:"domain_id" json:"domain_id"`
}

type Invite struct {
	ID               string         `sql:"id" json:"id"`
	ConversationID   string         `sql:"conversation_id" json:"conversation_id"`
	UserID           int64          `sql:"user_id" json:"user_id"`
	Title            sql.NullString `sql:"title" json:"title,omitempty"`
	TimeoutSec       int64          `sql:"timeout_sec" json:"timeout_sec"`
	InviterChannelID sql.NullString `sql:"inviter_channel_id" json:"inviter_channel_id,omitempty"`
	ClosedAt         sql.NullTime   `sql:"closed_at" json:"closed_at,omitempty"`
	CreatedAt        sql.NullTime   `sql:"created_at" json:"created_at,omitempty"`
	DomainID         int64          `sql:"domain_id" json:"domain_id"`
}

type Message struct {
	ID             int64          `sql:"id" json:"id"`
	ChannelID      sql.NullString `sql:"channel_id" json:"channel_id,omitempty"`
	ConversationID string         `sql:"conversation_id" json:"conversation_id"`
	Text           sql.NullString `sql:"text" json:"text,omitempty"`
	CreatedAt      sql.NullTime   `sql:"created_at" json:"created_at,omitempty"`
	UpdatedAt      sql.NullTime   `sql:"updated_at" json:"updated_at,omitempty"`
	Type           string         `sql:"type" json:"type"`
}

type Profile struct {
	ID        int64          `sql:"id" json:"id"`
	Name      string         `sql:"name" json:"name"`
	SchemaID  sql.NullInt64  `sql:"schema_id" json:"schema_id,omitempty"`
	Type      string         `sql:"type" json:"type"`
	Variables types.JSONText `sql:"variables" json:"variables"`
	DomainID  int64          `sql:"domain_id" json:"domain_id"`
}
