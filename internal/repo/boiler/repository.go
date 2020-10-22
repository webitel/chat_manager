package boilrepo

// import (
// 	"context"
// 	"database/sql"

// 	pb "github.com/webitel/protos/pkg/chat"
// 	"github.com/webitel/chat_manager/models"

// 	"github.com/rs/zerolog"
// 	"github.com/volatiletech/sqlboiler/v4/boil"
// )

// type Repository interface {
// 	ProfileRepository
// 	ConversationRepository
// 	ChannelRepository
// 	ClientRepository
// 	InviteRepository
// 	MessageRepository
// 	WithTransaction(txFunc func(*sql.Tx) error) (err error)
// 	CreateConversationTx(ctx context.Context, tx boil.ContextExecutor, c *models.Conversation) error
// 	CreateMessageTx(ctx context.Context, tx boil.ContextExecutor, m *models.Message) error
// 	GetChannelByIDTx(ctx context.Context, tx boil.ContextExecutor, id string) (*models.Channel, error)
// 	GetChannelsTx(
// 		ctx context.Context,
// 		tx boil.ContextExecutor,
// 		userID *int64,
// 		conversationID *string,
// 		connection *string,
// 		internal *bool,
// 		exceptID *string,
// 	) ([]*models.Channel, error)
// 	CloseChannelTx(ctx context.Context, tx boil.ContextExecutor, id string) error
// 	CreateChannelTx(ctx context.Context, tx boil.ContextExecutor, c *models.Channel) error
// 	CloseChannelsTx(ctx context.Context, tx boil.ContextExecutor, conversationID string) error
// 	CloseInviteTx(ctx context.Context, tx boil.ContextExecutor, inviteID string) error
// }

// type ProfileRepository interface {
// 	GetProfileByID(ctx context.Context, id int64) (*models.Profile, error)
// 	GetProfiles(ctx context.Context, id int64, size, page int32, fields, sort []string, profileType string, domainID int64) ([]*models.Profile, error)
// 	CreateProfile(ctx context.Context, p *models.Profile) error
// 	UpdateProfile(ctx context.Context, p *models.Profile) error
// 	DeleteProfile(ctx context.Context, id int64) error
// }

// type ConversationRepository interface {
// 	CloseConversation(ctx context.Context, id string) error
// 	GetConversations(
// 		ctx context.Context,
// 		id string,
// 		size int32,
// 		page int32,
// 		fields []string,
// 		sort []string,
// 		domainID int64,
// 		active bool,
// 		userID int64,
// 	) ([]*pb.Conversation, error)
// 	CreateConversation(ctx context.Context, c *models.Conversation) error
// 	GetConversationByID(ctx context.Context, id string) (*pb.Conversation, error)
// }

// type ChannelRepository interface {
// 	CloseChannel(ctx context.Context, id string) (*models.Channel, error)
// 	CloseChannels(ctx context.Context, conversationID string) error
// 	GetChannels(
// 		ctx context.Context,
// 		userID *int64,
// 		conversationID *string,
// 		connection *string,
// 		internal *bool,
// 		exceptID *string,
// 	) ([]*models.Channel, error)
// 	CreateChannel(ctx context.Context, c *models.Channel) error
// 	GetChannelByID(ctx context.Context, id string) (*models.Channel, error)
// }

// type ClientRepository interface {
// 	GetClientByID(ctx context.Context, id int64) (*models.Client, error)
// 	GetClientByExternalID(ctx context.Context, externalID string) (*models.Client, error)
// 	CreateClient(ctx context.Context, c *models.Client) error
// 	GetClients(ctx context.Context, limit, offset int) ([]*models.Client, error)
// }

// type InviteRepository interface {
// 	CreateInvite(ctx context.Context, m *models.Invite) error
// 	CloseInvite(ctx context.Context, inviteID string) error
// 	GetInviteByID(ctx context.Context, id string) (*models.Invite, error)
// }

// type MessageRepository interface {
// 	CreateMessage(ctx context.Context, m *models.Message) error
// 	GetMessages(ctx context.Context, id int64, size, page int32, fields, sort []string, conversationID string) ([]*models.Message, error)
// }

// type boilerRepository struct {
// 	db  *sql.DB
// 	log *zerolog.Logger
// }

// func NewRepository(db *sql.DB, log *zerolog.Logger) Repository {
// 	return &boilerRepository{
// 		db,
// 		log,
// 	}
// }
