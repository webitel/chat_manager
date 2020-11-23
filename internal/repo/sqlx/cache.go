package sqlxrepo

import (
	
	"database/sql"
	"github.com/jackc/pgtype"
)

func (repo *sqlxRepository) WriteConversationNode(conversationID string, nodeID string) error {
	_, err := repo.db.Exec(
		`insert into chat.conversation_node (conversation_id, node_id) values ($1, $2)` +
		` on conflict (conversation_id) do update set node_id = EXCLUDED.node_id`,
		conversationID, nodeID,
	)
	return err
}

func (repo *sqlxRepository) ReadConversationNode(conversationID string) (string, error) {
	var nodeID pgtype.Text
	// perform
	err := repo.db.Get(
		// result
		&nodeID,
		// query
		`select e.node_id from chat.conversation_node e where e.conversation_id=$1`,
		// params
		conversationID,
	)
	if err == sql.ErrNoRows {
		return "", nil // NOT Found !
	}
	if nodeID.Status != pgtype.Present {
		return "", err // -ERR -or- NOT Found
	}
	return nodeID.String, nil // +OK
}

func (repo *sqlxRepository) DeleteConversationNode(conversationID string) error {
	_, err := repo.db.Exec(
		`delete from chat.conversation_node e where e.conversation_id=$1`,
		conversationID,
	)
	return err
}

func (repo *sqlxRepository) ReadConfirmation(conversationID string) (string, error) {
	var messageWaitToken pgtype.Text
	// perform
	err := repo.db.Get(
		// result
		&messageWaitToken,
		// query
		`select e.confirmation_id from chat.conversation_confirmation e where e.conversation_id=$1`,
		// params
		conversationID,
	)
	if err == sql.ErrNoRows {
		return "", nil // NOT Found !
	}
	if messageWaitToken.Status != pgtype.Present {
		return "", err
	}
	return messageWaitToken.String, err
}

func (repo *sqlxRepository) WriteConfirmation(conversationID string, confirmationID string) error {
	_, err := repo.db.Exec(
		// query
		`insert into chat.conversation_confirmation (conversation_id, confirmation_id) values ($1, $2)`+
		` on conflict (conversation_id) do update set confirmation_id = EXCLUDED.confirmation_id`,
		// params ...
		conversationID, confirmationID,
	)
	return err
}

func (repo *sqlxRepository) DeleteConfirmation(conversationID string) error {
	_, err := repo.db.Exec(
		// query
		`delete from chat.conversation_confirmation where conversation_id=$1`,
		// params ...
		conversationID,
	)
	return err
}
