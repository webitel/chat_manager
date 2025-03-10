package sqlxrepo

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/pkg/errors"
	"github.com/webitel/chat_manager/internal/contact"
	"github.com/webitel/chat_manager/internal/repo/sqlx/proto"

	"github.com/jmoiron/sqlx/types"
)

var (
	channelAllColumns      = []string{"id", "type", "conversation_id", "user_id", "connection", "created_at", "internal", "closed_at", "updated_at", "domain_id", "flow_bridge", "name"}
	clientAllColumns       = []string{"id", "name", "number", "created_at", "activity_at", "external_id", "first_name", "last_name"}
	conversationAllColumns = []string{"id", "title", "created_at", "closed_at", "updated_at", "domain_id"}
	inviteAllColumns       = []string{"id", "conversation_id", "user_id", "title", "timeout_sec", "inviter_channel_id", "closed_at", "created_at", "domain_id"}
	messageAllColumns      = []string{"id", "channel_id", "conversation_id", "text", "created_at", "updated_at", "type"}
	profileAllColumns      = []string{"id", "url_id", "name", "schema_id", "type", "variables", "domain_id", "created_at"}
)

type Channel struct {
	ID             string         `db:"id" json:"id"`
	Type           string         `db:"type" json:"type"`
	ConversationID string         `db:"conversation_id" json:"conversation_id"`
	UserID         int64          `db:"user_id" json:"user_id"`
	Connection     sql.NullString `db:"connection" json:"connection,omitempty"`
	ServiceHost    sql.NullString `db:"host" json:"host,omitempty"`
	CreatedAt      time.Time      `db:"created_at" json:"created_at,omitempty"`
	Internal       bool           `db:"internal" json:"internal"`
	ClosedAt       sql.NullTime   `db:"closed_at" json:"closed_at,omitempty"`
	UpdatedAt      time.Time      `db:"updated_at" json:"updated_at,omitempty"`
	DomainID       int64          `db:"domain_id" json:"domain_id"`
	// FlowID         int64          `db:"-" json:"flow_id"`
	FlowBridge  bool           `db:"flow_bridge" json:"flow_bridge"`
	Name        string         `db:"name" json:"name"`
	PublicName  sql.NullString `db:"public_name" json:"public_name"`
	ClosedCause sql.NullString `db:"closed_cause" json:"closed_cause,omitempty"`
	JoinedAt    sql.NullTime   `db:"joined_at" json:"joined_at,omitempty"`
	Variables   Metadata       `db:"props" json:"props,omitempty"`
}

func (m Channel) FullName() string {
	fullName := m.Name
	if m.PublicName.Valid && m.PublicName.String != "" {
		fullName = m.PublicName.String
	}
	return fullName
}

func (m *Channel) Contact() string {
	// default: NULL
	contact, err := contact.NodeServiceContact(
		m.ServiceHost.String, m.Connection.String,
	)

	if err != nil {
		return m.Connection.String
	}

	return contact
}

func (m *Channel) ScanContact(src interface{}) error {
	// default: NULL
	// m.ServiceHost = ""
	m.Connection = sql.NullString{}

	if src == nil {
		return nil // NULL
	}

	dst := &m.Connection
	err := dst.Scan(src)

	if err != nil {
		// return errors.Wrap(err, "sql: scan "
		return err
	}

	if !dst.Valid {
		return nil // NULL
	}

	// normalize
	m.Connection.String, m.ServiceHost.String =
		contact.ContactServiceNode(m.Connection.String)

	m.Connection.Valid = m.Connection.String != ""

	return nil
}

func (m *Channel) ScanHostname(src interface{}) error {
	// default: CURRENT

	if src == nil {
		return nil // CURRENT
	}

	// err := ScanString(&m.ServiceHost).Scan(src)
	err := m.ServiceHost.Scan(src)

	if err != nil {
		// m.ServiceHost = "" // NULL-ify
		return err
	}

	m.Connection.String, _ =
		contact.ContactServiceNode(m.Connection.String)

	return nil
}

// func (m *Channel) scan(row *sql.Rows, ava []interface{}) error {

func (m *Channel) Scan(row *sql.Rows) error {

	// // dataset.next(?)
	// if !row.Next() {
	// 	return row.Err()
	// }

	cols, err := row.Columns()
	if err != nil {
		return err
	}

	dst := make([]interface{}, 0, len(cols))
	target := func(bind interface{}) {
		dst = append(dst, bind)
	}
	for _, att := range cols {
		switch att {

		case "id":
			target(&m.ID)
		case "type":
			target(&m.Type)
		case "name":
			target(&m.Name)

		case "user_id":
			target(&m.UserID)
		case "domain_id":
			target(&m.DomainID)
		case "conversation_id":
			target(&m.ConversationID)

		case "connection":
			target(&m.Connection)
		// case "connection":      target(ScanFunc(m.ScanContact))
		case "hostname", "host":
			target(&m.ServiceHost)
		//case "hostname","host": target(ScanFunc(m.ScanHostname))
		case "internal":
			target(&m.Internal)

		case "created_at":
			target(&m.CreatedAt)
		case "updated_at":
			target(&m.UpdatedAt)

		case "joined_at":
			target(&m.JoinedAt)
		case "closed_at":
			target(&m.ClosedAt)
		case "closed_cause":
			target(&m.ClosedCause)

		case "flow_bridge":
			target(&m.FlowBridge)

		case "props":
			target(&m.Variables)

		case "public_name":
			target(&m.PublicName)

		default:

			return errors.Errorf("sql: scan %T column %q not supported", m, att)

		}
	}

	err = row.Scan(dst...)

	// POST-Scan normalization here !..

	return err
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
	Messages      []*Message // ConversationMessages
	MembersBytes  []byte     `db:"members" json:"members"`
	MessagesBytes []byte     `db:"messages" json:"messages"`
	Variables     Metadata   `db:"vars" json:"variables"`
}

type ConversationMembers []*ConversationMember

type ConversationMember struct {
	ID        string    `db:"id" json:"id"`                   // channel.id
	Type      string    `db:"type" json:"type"`               // channel.type
	Name      string    `db:"name" json:"name"`               // user.name
	UserID    int64     `db:"user_id" json:"user_id"`         // user.id
	ChatID    string    `db:"external_id" json:"external_id"` // chat.id
	Internal  bool      `db:"internal" json:"internal"`       // (channel.type == webitel) ?
	CreatedAt time.Time `db:"created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at,omitempty"`
}

func (c *ConversationMembers) Scan(src interface{}) error {
	return json.Unmarshal(src.([]byte), c)
}

func (c *ConversationMembers) Value() (driver.Value, error) {
	return json.Marshal(c)
}

type ConversationMessages []*ConversationMessage

type ConversationMessage struct {
	ID        int64     `db:"id" json:"id"`
	ChannelID string    `db:"channel_id" json:"channel_id,omitempty"`
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
	Variables        Metadata       `db:"props" json:"props"`
}

type Document struct {
	ID   int64  `json:"id,omitempty"`
	URL  string `json:"url,omitempty"`
	Type string `json:"type,omitempty"`
	Size int64  `json:"size,omitempty"`
	Name string `json:"name,omitempty"`
}

type Message struct {
	ID int64 `db:"id" json:"id"`
	// ChannelID            sql.NullString `db:"channel_id" json:"channel_id,omitempty"`
	ChannelID string `db:"channel_id" json:"channel_id,omitempty"`
	//UserID         sql.NullInt64  `db:"user_id" json:"user_id,omitempty"`
	//UserType       sql.NullString `db:"user_type" json:"user_type,omitempty"`
	ConversationID string `db:"conversation_id" json:"conversation_id"`

	CreatedAt time.Time `db:"created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at,omitempty"`

	Type string `db:"type" json:"type"`
	// Text                 sql.NullString `db:"text" json:"text,omitempty"`
	Text                 string    `db:"text" json:"text,omitempty"`
	File                 *Document `db:"-"    json:"file,omitempty"`
	proto.Content                  // embedded
	ReplyToMessageID     int64     `db:"-"    json:"reply_to_message_id,omitempty"`
	ForwardFromMessageID int64     `db:"-"    json:"forward_from_message_id,omitempty"`
	// TODO: Variables map[string]string
	// Variables            types.JSONText `db:"variables" json:"variables,omitempty"`
	Variables Metadata `db:"variables" json:"variables,omitempty"`
}

type Profile struct {
	ID        int64          `db:"id" json:"id"`
	UrlID     string         `db:"url_id" json:"url_id"`
	Name      string         `db:"name" json:"name"`
	SchemaID  sql.NullInt64  `db:"schema_id" json:"schema_id,omitempty"`
	Type      string         `db:"type" json:"type"`
	Variables types.JSONText `db:"variables" json:"variables"`
	DomainID  int64          `db:"domain_id" json:"domain_id"`
	CreatedAt time.Time      `db:"created_at" json:"created_at,omitempty"`
}

type WebitelUser struct {
	ID       int64  `db:"id" json:"id"`
	Name     string `db:"name" json:"name"`
	DomainID int64  `db:"dc" json:"dc"`
	ChatName string `db:"chat_name" json:"chat_name"`
}

type ConversationNode struct {
	ConversationID string `db:"conversation_id" json:"conversation_id"`
	NodeID         string `db:"node_id" json:"node_id"`
}

type ConversationConfirmation struct {
	ConversationID string `db:"conversation_id" json:"conversation_id"`
	ConfirmationID string `db:"confirmation_id" json:"confirmation_id"`
}

type AppUser struct {
	ID        string `db:"id" json:"id"`
	DomainID  int64  `db:"domain_id" json:"domain_id"`
	AppID     string `db:"app_id" json:"app_id"`
	ServiceID string `db:"service_id" json:"service_id"`
}

type ChatBot struct {
	ID        int64     `db:"id" json:"id"`
	DomainID  int64     `db:"dc" json:"dc"`
	Name      string    `db:"name" json:"name"`
	FlowID    int64     `db:"flow_id" json:"flow_id"`
	Enabled   bool      `db:"enabled" json:"enabled"`
	Provider  string    `db:"provider" json:"provider"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}
