package sqlxrepo

func (repo *sqlxRepository) WriteConversationNode(conversationID string, nodeID string) error {
	_, err := repo.db.Exec(`insert into chat.conversation_node (conversation_id, node_id) values ($1, $2) 
		on conflict (conversation_id) do update set node_id = EXCLUDED.node_id`,
		conversationID, nodeID)
	return err
}

func (repo *sqlxRepository) ReadConversationNode(conversationID string) (string, error) {
	result := &ConversationNode{}
	err := repo.db.Get(result, `select * from chat.conversation_node where conversation_id=$1`, conversationID)
	return result.NodeID, err
}

func (repo *sqlxRepository) DeleteConversationNode(conversationID string) error {
	_, err := repo.db.Exec(`delete from chat.conversation_node where conversation_id=$1`,
		conversationID)
	return err
}

func (repo *sqlxRepository) ReadConfirmation(conversationID string) (string, error) {
	result := &ConversationConfirmation{}
	err := repo.db.Get(result, `select * from chat.conversation_confirmation where conversation_id=$1`, conversationID)
	return result.ConfirmationID, err
}

func (repo *sqlxRepository) WriteConfirmation(conversationID string, confirmationID string) error {
	_, err := repo.db.Exec(`insert into chat.conversation_confirmation (conversation_id, confirmation_id) values ($1, $2)
		on conflict (conversation_id) do update set confirmation_id = EXCLUDED.confirmation_id`,
		conversationID, confirmationID)
	return err
}

func (repo *sqlxRepository) DeleteConfirmation(conversationID string) error {
	_, err := repo.db.Exec(`delete from chat.conversation_confirmation where conversation_id=$1`,
		conversationID)
	return err
}
