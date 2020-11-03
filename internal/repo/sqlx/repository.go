package sqlxrepo

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

type Repository interface {
	ProfileRepository
	ConversationRepository
	ChannelRepository
	ClientRepository
	InviteRepository
	MessageRepository
	CacheRepository
	GetWebitelUserByID(ctx context.Context, id int64) (*WebitelUser, error)
	WithTransaction(txFunc func(*sqlx.Tx) error) (err error)
	CreateConversationTx(ctx context.Context, tx *sqlx.Tx, c *Conversation) error
	CreateMessageTx(ctx context.Context, tx *sqlx.Tx, m *Message) error
	GetChannelByIDTx(ctx context.Context, tx *sqlx.Tx, id string) (*Channel, error)
	GetChannelsTx(
		ctx context.Context,
		tx *sqlx.Tx,
		userID *int64,
		conversationID *string,
		connection *string,
		internal *bool,
		exceptID *string,
	) ([]*Channel, error)
	CreateChannelTx(ctx context.Context, tx *sqlx.Tx, c *Channel) error
	CloseChannelsTx(ctx context.Context, tx *sqlx.Tx, conversationID string) error
	CloseInviteTx(ctx context.Context, tx *sqlx.Tx, inviteID string) error
	CloseConversationTx(ctx context.Context, tx *sqlx.Tx, conversationID string) error
}

type ProfileRepository interface {
	GetProfileByID(ctx context.Context, id int64) (*Profile, error)
	GetProfiles(
		ctx context.Context,
		id int64,
		size int32,
		page int32,
		fields []string,
		sort []string,
		profileType string,
		domainID int64,
	) ([]*Profile, error)
	CreateProfile(ctx context.Context, p *Profile) error
	UpdateProfile(ctx context.Context, p *Profile) error
	DeleteProfile(ctx context.Context, id int64) error
}

type ConversationRepository interface {
	CloseConversation(ctx context.Context, id string) error
	GetConversations(
		ctx context.Context,
		id string,
		size int32,
		page int32,
		fields []string,
		sort []string,
		domainID int64,
		active bool,
		userID int64,
		messageSize int32,
	) ([]*Conversation, error)
	CreateConversation(ctx context.Context, c *Conversation) error
	GetConversationByID(ctx context.Context, id string) (*Conversation, error)
	//GetConversationByID(ctx context.Context, id string) (*Conversation, []*Channel, []*Message, error)
}

type ChannelRepository interface {
	CloseChannel(ctx context.Context, id string) (*Channel, error)
	CloseChannels(ctx context.Context, conversationID string) error
	GetChannels(
		ctx context.Context,
		userID *int64,
		conversationID *string,
		connection *string,
		internal *bool,
		exceptID *string,
	) ([]*Channel, error)
	CreateChannel(ctx context.Context, c *Channel) error
	GetChannelByID(ctx context.Context, id string) (*Channel, error)
	CheckUserChannel(ctx context.Context, channelID string, userID int64) (*Channel, error)
	UpdateChannel(ctx context.Context, channelID string) (int64, error)
}

type ClientRepository interface {
	GetClientByID(ctx context.Context, id int64) (*Client, error)
	GetClientByExternalID(ctx context.Context, externalID string) (*Client, error)
	CreateClient(ctx context.Context, c *Client) error
	// GetClients(limit, offset int) ([]*Client, error)
}

type InviteRepository interface {
	CreateInvite(ctx context.Context, m *Invite) error
	CloseInvite(ctx context.Context, inviteID string) error
	GetInviteByID(ctx context.Context, id string) (*Invite, error)
}

type MessageRepository interface {
	CreateMessage(ctx context.Context, m *Message) error
	GetMessages(
		ctx context.Context,
		id int64,
		size int32,
		page int32,
		fields []string,
		sort []string,
		domainID int64,
		conversationID string,
	) ([]*Message, error)
	GetLastMessage(conversationID string) (*Message, error)
}

type CacheRepository interface {
	WriteConversationNode(conversationID string, nodeID string) error
	ReadConversationNode(conversationID string) (string, error)
	DeleteConversationNode(conversationID string) error

	ReadConfirmation(conversationID string) (string, error)
	WriteConfirmation(conversationID string, confirmationID string) error
	DeleteConfirmation(conversationID string) error
}

type sqlxRepository struct {
	db  *sqlx.DB
	log *zerolog.Logger
}

func NewRepository(db *sqlx.DB, log *zerolog.Logger) Repository {
	return &sqlxRepository{
		db,
		log,
	}
}
