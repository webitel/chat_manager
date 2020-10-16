package boilrepo

// import (
// 	"context"
// 	"database/sql"
// 	"time"

// 	pb "github.com/matvoy/chat_server/api/proto/chat"
// 	"github.com/matvoy/chat_server/models"

// 	"github.com/google/uuid"
// 	"github.com/volatiletech/null/v8"
// 	"github.com/volatiletech/sqlboiler/v4/boil"
// 	"github.com/volatiletech/sqlboiler/v4/queries/qm"
// )

// func (r *boilerRepository) GetConversationByID(ctx context.Context, id string) (*pb.Conversation, error) {
// 	c, err := models.Conversations(
// 		models.ConversationWhere.ID.EQ(id),
// 		qm.Load(models.ConversationRels.Channels),
// 	).One(ctx, r.db)
// 	if err != nil {
// 		r.log.Warn().Msg(err.Error())
// 		if err == sql.ErrNoRows {
// 			return nil, nil
// 		}
// 		return nil, err
// 	}
// 	members := make([]*pb.Member, 0, len(c.R.Channels))
// 	for _, ch := range c.R.Channels {
// 		if !ch.Internal {
// 			client, err := models.Clients(models.ClientWhere.ID.EQ(ch.UserID)).One(ctx, r.db)
// 			if err != nil {
// 				return nil, err
// 			}
// 			members = append(members, &pb.Member{
// 				ChannelId: ch.ID,
// 				UserId:    ch.UserID,
// 				Type:      ch.Type,
// 				Username:  client.Name.String,
// 				Firstname: client.FirstName.String,
// 				Lastname:  client.LastName.String,
// 				Internal:  ch.Internal,
// 			})
// 		} else {
// 			members = append(members, &pb.Member{
// 				ChannelId: ch.ID,
// 				UserId:    ch.UserID,
// 				Type:      ch.Type,
// 				Internal:  ch.Internal,
// 			})
// 		}

// 	}
// 	conv := &pb.Conversation{
// 		Id:        c.ID,
// 		Title:     c.Title.String,
// 		CreatedAt: c.CreatedAt.Time.Unix() * 1000,
// 		DomainId:  c.DomainID,
// 		Members:   members,
// 	}
// 	if c.ClosedAt != (null.Time{}) {
// 		conv.ClosedAt = c.ClosedAt.Time.Unix() * 1000
// 	}
// 	if c.UpdatedAt != (null.Time{}) {
// 		conv.UpdatedAt = c.UpdatedAt.Time.Unix() * 1000
// 	}
// 	return conv, nil
// }

// func (r *boilerRepository) CreateConversation(ctx context.Context, c *models.Conversation) error {
// 	c.ID = uuid.New().String()
// 	if err := c.Insert(ctx, r.db, boil.Infer()); err != nil {
// 		return err
// 	}
// 	return nil
// }

// func (r *boilerRepository) CloseConversation(ctx context.Context, id string) error {
// 	// result, err := models.Conversations(qm.Where("LOWER(session_id) like ?", strings.ToLower(sessionID)), qm.Where("closed_at is null")).
// 	// 	One(ctx, repo.db)
// 	result, err := models.Conversations(models.ConversationWhere.ID.EQ(id)).
// 		One(ctx, r.db)
// 	if err != nil {
// 		r.log.Warn().Msg(err.Error())
// 		if err == sql.ErrNoRows {
// 			return nil
// 		}
// 		return err
// 	}
// 	result.ClosedAt = null.Time{
// 		Valid: true,
// 		Time:  time.Now(),
// 	}
// 	_, err = result.Update(ctx, r.db, boil.Infer())
// 	return err
// }

// func (r *boilerRepository) GetConversations(
// 	ctx context.Context,
// 	id string,
// 	size int32,
// 	page int32,
// 	fields []string,
// 	sort []string,
// 	domainID int64,
// 	active bool,
// 	userID int64,
// ) ([]*pb.Conversation, error) {
// 	query := make([]qm.QueryMod, 0, 9)
// 	query = append(query, qm.Load(models.ConversationRels.Channels))
// 	if size != 0 {
// 		query = append(query, qm.Limit(int(size)))
// 	} else {
// 		query = append(query, qm.Limit(15))
// 	}
// 	if page != 0 {
// 		query = append(query, qm.Offset(int((page-1)*size)))
// 	}
// 	if id != "" {
// 		query = append(query, models.ConversationWhere.ID.EQ(id))
// 	}
// 	if fields != nil && len(fields) > 0 {
// 		query = append(query, qm.Select(fields...))
// 	}
// 	if sort != nil && len(sort) > 0 {
// 		for _, item := range sort {
// 			query = append(query, qm.OrderBy(item))
// 		}
// 	}
// 	if domainID != 0 {
// 		query = append(query, models.ConversationWhere.DomainID.EQ(domainID))
// 	}
// 	if active {
// 		query = append(query, models.ConversationWhere.ClosedAt.IsNull())
// 	}
// 	if userID != 0 {
// 		channels, err := models.Channels(
// 			models.ChannelWhere.UserID.EQ(userID),
// 			qm.Select("conversation_id"),
// 			qm.Distinct("conversation_id"),
// 		).All(ctx, r.db)
// 		if err != nil {
// 			return nil, err
// 		}
// 		if len(channels) > 0 {
// 			ids := make([]string, 0, len(channels))
// 			for _, item := range channels {
// 				ids = append(ids, item.ConversationID)
// 			}
// 			query = append(query, models.ConversationWhere.ID.IN(ids))
// 		}
// 	}
// 	// var conversations models.ConversationSlice
// 	// err := queries.Raw(`select c.*
// 	// from chat.conversation c
// 	// where c.id in (
// 	// 	select distinct ch.conversation_id
// 	// 	from chat.channel ch
// 	// 	where ch.user_id = 10
// 	// ) `).Bind(ctx, r.db, &conversations)
// 	conversations, err := models.Conversations(query...).All(ctx, r.db)
// 	if err != nil {
// 		return nil, err
// 	}
// 	result := make([]*pb.Conversation, 0, len(conversations))
// 	for _, c := range conversations {
// 		if len(c.R.Channels) == 0 {
// 			continue
// 		}
// 		members := make([]*pb.Member, 0, len(c.R.Channels))
// 		for _, ch := range c.R.Channels {
// 			if !ch.Internal {
// 				client, err := models.Clients(models.ClientWhere.ID.EQ(ch.UserID)).One(ctx, r.db)
// 				if err != nil {
// 					return nil, err
// 				}
// 				members = append(members, &pb.Member{
// 					ChannelId: ch.ID,
// 					UserId:    ch.UserID,
// 					Type:      ch.Type,
// 					Username:  client.Name.String,
// 					Firstname: client.FirstName.String,
// 					Lastname:  client.LastName.String,
// 				})
// 			} else {
// 				members = append(members, &pb.Member{
// 					ChannelId: ch.ID,
// 					UserId:    ch.UserID,
// 				})
// 			}
// 		}
// 		conv := &pb.Conversation{
// 			Id:        c.ID,
// 			Title:     c.Title.String,
// 			CreatedAt: c.CreatedAt.Time.Unix() * 1000,
// 			DomainId:  c.DomainID,
// 			Members:   members,
// 		}
// 		if c.ClosedAt != (null.Time{}) {
// 			conv.ClosedAt = c.ClosedAt.Time.Unix() * 1000
// 		}
// 		if c.UpdatedAt != (null.Time{}) {
// 			conv.UpdatedAt = c.UpdatedAt.Time.Unix() * 1000
// 		}
// 		result = append(result, conv)
// 	}
// 	return result, nil
// }
