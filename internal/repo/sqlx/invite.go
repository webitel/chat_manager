package sqlxrepo

import (
	"context"
	"strings"
	"time"

	"database/sql"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgtype"
	"github.com/jmoiron/sqlx"

	"github.com/google/uuid"

	errs "github.com/pkg/errors"

	"github.com/webitel/chat_manager/app"
)

// InviteList scan sql.Rows dataset tuples.
// Zero or negative `limit` implies NOLIMIT startegy.
// MAY: Return len([]*limit) == (size+1)
// which indicates that .`next` result page exist !
func InviteList(rows *sql.Rows, limit int) ([]*Invite, error) {

	//
	if limit < 0 {
		limit = 0
	}

	// TODO: prepare projection
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	// alloc projection map
	var (
		obj  *Invite                         // cursor: target for current tuple
		plan = make([]func() any, len(cols)) // , len(cols))
	)

	for c, col := range cols {
		switch col {
		// id, inviter_channel_id, conversation_id, title, user_id, domain_id, timeout_sec, created_at, closed_at, props
		case "id":
			plan[c] = func() any { return &obj.ID } // NOTNULL (!)
		case "conversation_id":
			plan[c] = func() any { return &obj.ConversationID } // NOTNULL (!)
		case "inviter_channel_id":
			plan[c] = func() any { return &obj.InviterChannelID } // NULL: *sql.NullString

		case "title":
			plan[c] = func() any { return &obj.Title } // NULL: *sql.NullString
		case "user_id":
			plan[c] = func() any { return &obj.UserID } // NOTNULL: (!)
		case "domain_id":
			plan[c] = func() any { return &obj.DomainID } // NOTNULL: (!)

		case "created_at":
			plan[c] = func() any { return ScanDatetime(&obj.CreatedAt) } // NOTNULL (!)
		case "closed_at":
			plan[c] = func() any { return &obj.ClosedAt } // NULL: *sql.NullTime

		case "timeout_sec":
			plan[c] = func() any { return ScanInteger(&obj.TimeoutSec) } // NULL: (!)
		case "props":
			plan[c] = func() any { return &obj.Variables } // NULL: *Properties

		default:

			return nil, errs.Errorf("sql: scan %T column %q not supported", obj, col)

		}
	}

	dst := make([]interface{}, len(cols))

	var (
		page []Invite  // mempage
		list []*Invite // results
	)

	if limit > 0 {

		page = make([]Invite, limit)
		list = make([]*Invite, 0, limit+1)

	}

	// var (

	// 	err error
	// 	row *Message
	// )

	for rows.Next() {

		if 0 < limit && len(list) == limit {
			// indicate next page exists !
			// rows.Next(!)
			list = append(list, nil)
			break
		}

		if len(page) != 0 {

			obj = &page[0]
			page = page[1:]

		} else {

			obj = new(Invite)
		}

		for c, bind := range plan {
			dst[c] = bind()
		}

		err = rows.Scan(dst...)

		if err != nil {
			break
		}

		// // region: check file document attached
		// if doc.ID != 0 {
		// 	obj.File, doc = doc, nil
		// }
		// // endregion

		list = append(list, obj)

	}

	if err == nil {
		err = rows.Err()
	}

	if err != nil {
		return nil, err
	}

	return list, nil
}

func schemaInviteError(err error) error {
	if err == nil {
		return nil
	}
	switch err.(type) {
	case *pgconn.PgError:
		// TODO: handle shema-specific errors, constraints, violations ...
	}
	return err
}

func (repo *sqlxRepository) GetInviteByID(ctx context.Context, id string) (*Invite, error) {

	var inviteId pgtype.UUID
	err := inviteId.Set(id)
	if err != nil {
		// ERR: getInvite( id: uuid! ); input: invalid or missing
		return nil, err
	}

	// Perform SELECT statement
	rows, err := repo.db.QueryContext(ctx,
		"SELECT id, inviter_channel_id, conversation_id, title, user_id, domain_id, timeout_sec, created_at, closed_at, props"+
			" FROM chat.invite WHERE id=$1 AND closed_at ISNULL"+
			" LIMIT 2", // NOTE: to be able to indicate result conflict(s)
		&inviteId,
	)
	// Check errors
	if err = schemaInviteError(err); err != nil {
		repo.log.Error().Err(err).Str("id", id).
			Msg("FAILED Lookup DB chat.invite")
		return nil, err
	}

	defer rows.Close()
	// Fetch results
	list, err := InviteList(rows, 1)

	if err != nil {
		repo.log.Error().Err(err).Str("id", id).
			Msg("FAILED Fetch DB chat.invite")
		return nil, err
	}

	var res *Invite
	if size := len(list); size != 0 {
		if size != 1 {
			// NOTE: page .next exists !
			// return nil, errors.Conflict(
			// 	"chat.channel.search.id.conflict",
			// 	"chat: got too much records looking for channel "+ id,
			// )
			return nil, errs.New("got too much records")
		}
		res = list[0]
	}

	if res == nil || !strings.EqualFold(id, res.ID) {
		res = nil // NOT FOUND !
	}

	return res, nil
}

/*func (repo *sqlxRepository) GetInviteByID(ctx context.Context, id string) (*Invite, error) {
	res := &Invite{}
	err := repo.db.GetContext(ctx, res,
		"SELECT * FROM chat.invite"+
		" WHERE id=$1 AND closed_at ISNULL",
		 id,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		repo.log.Warn().Msg(err.Error())
		return nil, err
	}
	return res, nil
}*/

func (repo *sqlxRepository) GetInvites(ctx context.Context, userID int64) ([]*Invite, error) {
	res := []*Invite{}
	err := repo.db.SelectContext(ctx, &res,
		"SELECT * FROM chat.invite WHERE user_id=$1",
		userID,
	)
	return res, err
}

func (repo *sqlxRepository) CreateInvite(ctx context.Context, m *Invite) (err error) {

	var (
		threadId, inviteId, senderId pgtype.UUID
		// isZero = func(uuid [16]byte) bool {
		// 	const size = 16 // bytes
		// 	for e := 0; e < size && uuid[e] != 0; e++ {
		// 		return false // HAS non-zero byte(!)
		// 	}
		// 	return true // ALL are zero(!)
		// }
	)

	err = threadId.Set(m.ConversationID)
	if err != nil {
		// ERR: invite( conversation_id: uuid! ); input: invalid or missing
		return err
	}
	if m.ID == "" {
		id := uuid.New()
		m.ID = id.String()
		_ = inviteId.Set([16]byte(id))
	} else {
		err = inviteId.Set(m.ID)
		if err != nil {
			// ERR: invite( id: uuid! ); input: invalid syntax
			return err
		}
	}
	senderId.Status = pgtype.Null
	if m.InviterChannelID.Valid {
		err = senderId.Set(
			m.InviterChannelID.String,
		)
		if err != nil {
			// ERR: invite( inviter_id: uuid! ); input: invalid syntax
			return err
		}
	}

	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	m.CreatedAt = m.CreatedAt.UTC()

	if m.Title.String == "" {
		// TODO: get .FROM inviter channel contact display name
		_, err = repo.db.ExecContext(ctx,
			"WITH sender AS ("+
				"SELECT COALESCE(contact.name, NULLIF(account.name,''), account.username, channel.name) AS display"+
				" FROM chat.channel"+
				" LEFT JOIN chat.client AS contact ON (contact.id, false) = (channel.user_id, channel.internal)"+
				" LEFT JOIN directory.wbt_user AS account ON (account.id, true) = (channel.user_id, channel.internal)"+
				" WHERE channel.id = $1"+
				") "+
				"INSERT INTO chat.invite ("+
				"id, conversation_id, user_id, title, timeout_sec, inviter_channel_id, created_at, domain_id, props"+
				") VALUES ($1, $2, $3, COALESCE((SELECT display FROM sender), 'noname'), $4, $5, $6, $7, $8)",
			// m.ID,
			&inviteId,
			// m.ConversationID,
			&threadId,
			m.UserID,
			m.TimeoutSec,
			// m.InviterChannelID,
			&senderId,
			m.CreatedAt,
			m.DomainID,
			NullMetadata(m.Variables),
		)

	} else { // typical logic }

		_, err = repo.db.ExecContext(ctx,
			"INSERT INTO chat.invite ("+
				"id, conversation_id, user_id, title, timeout_sec, inviter_channel_id, created_at, domain_id, props"+
				") VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
			// m.ID,
			&inviteId,
			// m.ConversationID,
			&threadId,
			m.UserID,
			m.Title,
			m.TimeoutSec,
			// m.InviterChannelID,
			&senderId,
			m.CreatedAt,
			m.DomainID,
			m.Variables,
		)
	}

	if err != nil {
		return err
	}

	return nil
}

func (repo *sqlxRepository) CloseInvite(ctx context.Context, inviteID string) (bool, error) {
	return CloseInvite(ctx, repo.db, inviteID)
}

func CloseInvite(ctx context.Context, dcx sqlx.ExtContext, inviteID string) (ok bool, err error) {

	var inviteId pgtype.UUID
	err = inviteId.Set(inviteID)
	if err != nil {
		// ERR: closeInvite( id: uuid! ); input: invalid or missing
		return false, err
	}

	err = sqlx.GetContext(ctx, dcx, &ok,
		"UPDATE chat.invite SET closed_at=$2"+
			" WHERE id=$1 AND closed_at ISNULL"+
			" RETURNING true", // Found AND Updated !
		&inviteId, app.CurrentTime(),
	)
	// Handle sqlx.Get* -specific error
	if err == sql.ErrNoRows {
		// ok = false // default: !
		err = nil
	}

	ok = ok && nil == err

	return ok, err
}
