package sqlxrepo

import (

	"fmt"
	"time"
	"strings"
	"context"

	"github.com/google/uuid"

	"database/sql"
	"github.com/jmoiron/sqlx"
)

//type StringIDs []string
//
//func (strs StringIDs) Value() (driver.Value, error) {
//	return strings.Join(strs, ", "), nil
//}

func (repo *sqlxRepository) GetConversationByID(ctx context.Context, id string) (*Conversation, error) {
	conversation := &Conversation{}
	err := repo.db.GetContext(ctx, conversation, "SELECT * FROM chat.conversation WHERE id=$1", id)
	if err != nil {
		repo.log.Warn().Msg(err.Error())
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	conversation.Members, conversation.Messages, err = repo.getConversationInfo(ctx, id)
	if err != nil {
		repo.log.Error().Msg(err.Error())
		return nil, err
	}
	return conversation, nil
}

func (repo *sqlxRepository) CreateConversation(ctx context.Context, session *Conversation) error {
	return NewSession(repo.db, ctx, session)
}

/*func (repo *sqlxRepository) CreateConversation(ctx context.Context, c *Conversation) error {
	c.ID = uuid.New().String()
	tmp := time.Now()
	c.CreatedAt = tmp
	c.UpdatedAt = tmp
	_, err := repo.db.NamedExecContext(ctx, `insert into chat.conversation (id, title, created_at, closed_at, updated_at, domain_id)
	values (:id, :title, :created_at, :closed_at, :updated_at, :domain_id)`, *c)
	return err
}*/

// TODO: CloseConversation(ctx context.Context, id string, at time.Time) error {}
func (repo *sqlxRepository) CloseConversation(ctx context.Context, id string) error {
	
	at := time.Now()
	
	// with cancellation context
	_, err := repo.db.ExecContext(ctx,
		// query statement
		psqlSessionCloseQ,
		// query params ...
		id, at.UTC(),
	)

	return err
}

/*func (repo *sqlxRepository) CloseConversation(ctx context.Context, id string) error {
	_, err := repo.db.ExecContext(ctx, `update chat.conversation set closed_at=$1 where id=$2`, sql.NullTime{
		Valid: true,
		Time:  time.Now(),
	}, id)
	return err
}*/

func (repo *sqlxRepository) GetConversations(
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
) ([]*Conversation, error) {
	conversations := make([]*Conversation, 0, size)
	fieldsStr, whereStr, sortStr, limitStr := "c.*, m.*, ch.*", "", "order by c.created_at desc", ""
	if size == 0 {
		size = 15
	}
	if page == 0 {
		page = 1
	}
	limitStr = fmt.Sprintf("limit %v offset %v", size, (page-1)*size)
	if messageSize == 0 {
		messageSize = 10
	}
	messageLimitStr := fmt.Sprintf("limit %v", messageSize)
	queryStrings := make([]string, 0, 4)
	queryArgs := make([]interface{}, 0, 4)
	argCounter := 1
	if userID != 0 {
		whereStr = "right join chat.channel rch on c.id = rch.conversation_id where rch.user_id=$1 and"
		queryArgs = append(queryArgs, userID)
		argCounter++
	}
	if id != "" {
		queryStrings = append(queryStrings, "c.id")
		queryArgs = append(queryArgs, id)
	}
	// TO DO GET DOMAIN FROM TOKEN
	if domainID != 0 {
		queryStrings = append(queryStrings, "c.domain_id")
		queryArgs = append(queryArgs, domainID)
	}
	if len(queryStrings) > 0 {
		if whereStr == "" {
			whereStr = "where"
		}
		if active != false {
			whereStr = whereStr + " c.closed_at is not null and"
		}
		for i, _ := range queryStrings {
			whereStr = whereStr + fmt.Sprintf(" %s=$%v and", queryStrings[i], i+argCounter)
		}
	}
	whereStr = strings.TrimRight(whereStr, " and")
	query := fmt.Sprintf(`
		select %s
			from chat.conversation c
				left join LATERAL (
					select json_agg(s) as messages
					from (
						SELECT
							   m.id,
							   m.text,
							   m.type,
							   m.channel_id,
							   m.created_at,
							   m.updated_at
						FROM chat.message m
						where m.conversation_id = c.id
						order by m.created_at desc
						%s
					) s
				) m on true
				left join LATERAL (
					select json_agg(ss) as members
					from (
						select
							   ch.id,
							   ch.type,
							   ch.user_id,
							   ch.name,
							   ch.internal,
							   ch.created_at,
							   ch.updated_at
						from chat.channel ch
						where ch.conversation_id = c.id
					) ss
				) ch on true
			%s
			%s
		%s;
		`, fieldsStr, messageLimitStr, whereStr, sortStr, limitStr)
	rows, err := repo.db.QueryxContext(ctx, query, queryArgs...)
	if err != nil {
		// if err == sql.ErrNoRows {
		// 	return nil, nil
		// }
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		tmp := new(Conversation)
		rows.StructScan(tmp)
		tmp.Members.Scan(tmp.MembersBytes)
		tmp.Messages.Scan(tmp.MessagesBytes)
		conversations = append(conversations, tmp)
	}
	return conversations, nil
}

func (repo *sqlxRepository) getConversationInfo(ctx context.Context, id string) (members ConversationMembers, messages ConversationMessages, err error) {
	members = ConversationMembers{}
	err = repo.db.SelectContext(ctx, &members,
		`select
			   ch.id,
			   ch.type,
			   ch.user_id,
			   ch.name,
			   ch.internal,
			   ch.created_at,
			   ch.updated_at
		from chat.channel ch
		where ch.conversation_id = $1`, id)
	if err != nil {
		repo.log.Warn().Msg(err.Error())
		// if err == sql.ErrNoRows {
		// 	err = nil
		// 	return
		// }
		return
	}
	messages = ConversationMessages{}
	err = repo.db.GetContext(ctx, &messages, `
		SELECT 
			   m.id,
			   m.text,
			   m.type,
			   m.channel_id,
			   m.created_at,
			   m.updated_at
		FROM chat.message m
		where m.conversation_id=$1
		order by m.created_at desc
		limit 10`, id)
	if err != nil {
		if err == sql.ErrNoRows {
			err = nil
			return
		}
		repo.log.Warn().Msg(err.Error())
		return
	}
	return
}

// NewSession creates NEW chat session DB record
func NewSession(dcx sqlx.ExtContext, ctx context.Context, session *Conversation) error {

	at := time.Now().UTC()
	// Generate NEW unique UUID for this brand NEW chat session
	session.ID = uuid.New().String()
	
	session.CreatedAt = at
	session.UpdatedAt = at

	// FIXME:
	session.Title.Valid = true // NOTNULL
	session.Title.String =
		strings.TrimSpace(
			session.Title.String,
		)

	_, err := dcx.ExecContext(
		// cancellation context
		ctx,
		// statement query
		psqlSessionNewQ,
		// statement params ...
		session.ID,
		session.DomainID,
		session.Title,

		session.CreatedAt,
		session.UpdatedAt,
		session.ClosedAt, // nil,
	)

	if err != nil {
		return err
	}
	// +OK
	return nil
}

// postgres: chat.session.close(!)
// $1 - conversation_id
// $2 - local timestamp
const psqlSessionCloseQ =
`WITH c0 AS (
  DELETE FROM chat.conversation_confirmation
   WHERE conversation_id=$1
), c1 AS (
  DELETE FROM chat.conversation_node
   WHERE conversation_id=$1
), c2 AS (
  UPDATE chat.conversation
     SET closed_at=$2
   WHERE id=$1
)
UPDATE chat.channel
   SET closed_at=$2
 WHERE conversation_id=$1
`

// postgres: chat.session.create(!)
// $1  - session.id
// $2  - session.domain_id
// $3  - session.title
// $4  - session.created_at
// $5  - session.updated_at
// $6  - session.closed_at // FIXME: NULL ?
const psqlSessionNewQ =
`INSERT INTO chat.conversation (
  id, domain_id, title, created_at, updated_at, closed_at
) VALUES (
  $1, $2, $3, $4, $5, $6
)`