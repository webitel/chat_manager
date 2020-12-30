package sqlxrepo

import (
	
	"fmt"
	"time"
	"context"
)

func (repo *sqlxRepository) CreateMessage(ctx context.Context, m *Message) error {

	now := time.Now()

	m.ID = 0
	m.CreatedAt = now
	m.UpdatedAt = now

	err := repo.db.GetContext(
		// context, result
		ctx, &m.ID, 
		// statement query !
		sentMessageQ,
		// statement params ...
		now.UTC(),        // $1 - SEND timestamp
		&m.ChannelID,     // $2 - FROM: sender channel_id
		m.ConversationID, // $3 - TO: session conversation_id
		m.Type,           // $4 - SEND: message event (default: text)
		&m.Text,          // $5 - SEND: message text
		m.Variables,      // $6 - SEND: message vars
	)

	if err != nil {
		return err
	}

	return nil
}

/*func (repo *sqlxRepository) CreateMessage(ctx context.Context, m *Message) error {
	m.ID = 0
	tmp := time.Now()
	m.CreatedAt = tmp
	m.UpdatedAt = tmp
	stmt, err := repo.db.PrepareNamed(`insert into chat.message (channel_id, conversation_id, text, created_at, updated_at, type)
	values (:channel_id, :conversation_id, :text, :created_at, :updated_at, :type) RETURNING id`)
	if err != nil {
		return err
	}
	var id int64
	err = stmt.GetContext(ctx, &id, *m)
	if err != nil {
		return err
	}
	m.ID = id
	_, err = repo.db.ExecContext(ctx, `update chat.conversation set updated_at=$1 where id=$2`, tmp, m.ConversationID)
	if err != nil {
		return err
	}
	_, err = repo.db.ExecContext(ctx, `update chat.channel set updated_at=$1 where id=$2`, tmp, m.ChannelID)
	return err
}*/

func (repo *sqlxRepository) GetMessages(ctx context.Context, id int64, size, page int32, fields, sort []string, domainID int64, conversationID string) ([]*Message, error) {
	result := []*Message{}
	fieldsStr, whereStr, sortStr, limitStr := 
		"m.id, m.channel_id, m.conversation_id, m.text, m.created_at, m.updated_at, m.type, m.variables, "+
		  "c.user_id, c.type as user_type",
		"where c.domain_id=$1 and m.conversation_id=$2",
		"order by created_at desc",
		""
	if size == 0 {
		size = 15
	}
	if page == 0 {
		page = 1
	}
	limitStr = fmt.Sprintf("limit %d offset %d", size, (page-1)*size)
	query := fmt.Sprintf("SELECT %s FROM chat.message m left join chat.channel c on m.channel_id = c.id %s %s %s", fieldsStr, whereStr, sortStr, limitStr)
	err := repo.db.SelectContext(ctx, &result, query, domainID, conversationID)
	return result, err
}

func (repo *sqlxRepository) GetLastMessage(conversationID string) (*Message, error) {
	result := &Message{}
	err := repo.db.Get(result, "select id, text, variables from chat.message where conversation_id=$1 order by created_at desc limit 1", conversationID)
	return result, err
}

// Statement to store historical (SENT) message
// $1 - SEND timestamp
// $2 - FROM: sender channel_id
// $3 - TO: session conversation_id
// $4 - SEND: message event (default: text)
// $5 - SEND: message text
// $6 - SEND: message vars
const sentMessageQ = 
`WITH sender AS (UPDATE chat.channel SET updated_at=$1 WHERE id=$2)
, latest AS (UPDATE chat.conversation SET updated_at=$1 WHERE id=$3)
INSERT INTO chat.message (
  created_at, updated_at, channel_id, conversation_id, type, text, variables
) VALUES (
  $1, $1, $2, $3, $4, $5, $6
) RETURNING id`