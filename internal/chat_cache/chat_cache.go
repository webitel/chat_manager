package chat_cache

import (
	"fmt"
	"time"

	"github.com/micro/go-micro/v2/store"
)

const (
	sessionStr            = "session_id:%s"         // %s - session id, value - conversation id
	confirmationStr       = "confirmations:%v"      // %s - conversation id, value - confirmation id
	writeCachedMessageStr = "cached_messages:%v:%v" // %v - conversation id, %v - message id
	readCachedMessageStr  = "cached_messages:%v"    // %v - conversation id
	conversationNodeStr   = "conversation:%v:node"  // %v - conversation id
	userInfoStr           = "userinfo:%s"           // %s - token
)

type ChatCache interface {
	ReadSession(sessionID string) ([]byte, error)
	WriteSession(sessionID string, conversationIDBytes []byte) error
	DeleteSession(sessionID string) error

	WriteConversationNode(conversationID string, nodeIDBytes []byte) error
	ReadConversationNode(conversationID string) ([]byte, error)
	DeleteConversationNode(conversationID string) error

	ReadConfirmation(conversationID string) ([]byte, error)
	WriteConfirmation(conversationID string, confirmationIDBytes []byte) error
	DeleteConfirmation(conversationID string) error

	ReadCachedMessages(conversationID string) ([]*store.Record, error)
	WriteCachedMessage(conversationID string, messageID int64, messageBytes []byte) error
	DeleteCachedMessages(conversationID string) error
	DeleteCachedMessage(key string) error

	SetUserInfo(token string, infoBytes []byte, expires int64) error
	GetUserInfo(token string) (bool, error)
}

type chatCache struct {
	redisStore store.Store
}

func NewChatCache(redisStore store.Store) ChatCache {
	return &chatCache{
		redisStore,
	}
}

func (c *chatCache) SetUserInfo(token string, infoBytes []byte, expires int64) error {
	key := fmt.Sprintf(userInfoStr, token)
	return c.redisStore.Write(&store.Record{
		Key:    key,
		Value:  infoBytes,
		Expiry: time.Millisecond * time.Duration(expires-time.Now().Unix()*1000),
	})
}

func (c *chatCache) GetUserInfo(token string) (bool, error) {
	key := fmt.Sprintf(userInfoStr, token)
	info, err := c.redisStore.Read(key)
	if err != nil && err.Error() != "not found" {
		return false, err
	}
	if len(info) > 0 {
		return true, nil
	} else {
		return false, nil
	}
}

func (c *chatCache) ReadSession(sessionID string) ([]byte, error) {
	sessionKey := fmt.Sprintf(sessionStr, sessionID)
	session, err := c.redisStore.Read(sessionKey)
	if err != nil && err.Error() != "not found" {
		return nil, err
	}
	if len(session) > 0 {
		return session[0].Value, nil
	} else {
		return nil, nil
	}
}

func (c *chatCache) WriteSession(sessionID string, conversationIDBytes []byte) error {
	sessionKey := fmt.Sprintf(sessionStr, sessionID)
	return c.redisStore.Write(&store.Record{
		Key:    sessionKey,
		Value:  conversationIDBytes,
		Expiry: time.Hour * time.Duration(24),
	})
}

func (c *chatCache) WriteConversationNode(conversationID string, nodeIDBytes []byte) error {
	key := fmt.Sprintf(conversationNodeStr, conversationID)
	return c.redisStore.Write(&store.Record{
		Key:   key,
		Value: nodeIDBytes,
		// Expiry: time.Hour * time.Duration(24),
	})
}

func (c *chatCache) ReadConversationNode(conversationID string) ([]byte, error) {
	key := fmt.Sprintf(conversationNodeStr, conversationID)
	node, err := c.redisStore.Read(key)
	if err != nil && err.Error() != "not found" {
		return nil, err
	}
	if len(node) > 0 {
		return node[0].Value, nil
	} else {
		return nil, nil
	}
}

func (c *chatCache) DeleteConversationNode(conversationID string) error {
	key := fmt.Sprintf(conversationNodeStr, conversationID)
	return c.redisStore.Delete(key)
}

func (c *chatCache) DeleteSession(sessionID string) error {
	sessionKey := fmt.Sprintf(sessionStr, sessionID)
	return c.redisStore.Delete(sessionKey)
}

func (c *chatCache) ReadConfirmation(conversationID string) ([]byte, error) {
	confirmationKey := fmt.Sprintf(confirmationStr, conversationID)
	confirmationID, err := c.redisStore.Read(confirmationKey)
	if err != nil && err.Error() != "not found" {
		return nil, err
	}
	if len(confirmationID) > 0 {
		return confirmationID[0].Value, nil
	} else {
		return nil, nil
	}
}

func (c *chatCache) WriteConfirmation(conversationID string, confirmationIDBytes []byte) error {
	confirmationKey := fmt.Sprintf(confirmationStr, conversationID)
	return c.redisStore.Write(&store.Record{
		Key:    confirmationKey,
		Value:  confirmationIDBytes,
		Expiry: time.Hour * time.Duration(24),
	})
}

func (c *chatCache) DeleteConfirmation(conversationID string) error {
	confirmationKey := fmt.Sprintf(confirmationStr, conversationID)
	return c.redisStore.Delete(confirmationKey)
}

func (c *chatCache) ReadCachedMessages(conversationID string) ([]*store.Record, error) {
	messagesKey := fmt.Sprintf(readCachedMessageStr, conversationID)
	cachedMessages, err := c.redisStore.Read(messagesKey)
	if err != nil && err.Error() != "not found" {
		return nil, err
	}
	if len(cachedMessages) > 0 {
		return cachedMessages, nil
	} else {
		return nil, nil
	}
}

func (c *chatCache) WriteCachedMessage(conversationID string, messageID int64, messageBytes []byte) error {
	messagesKey := fmt.Sprintf(writeCachedMessageStr, conversationID, messageID)
	return c.redisStore.Write(&store.Record{
		Key:    messagesKey,
		Value:  messageBytes,
		Expiry: time.Hour * time.Duration(24),
	})
}

func (c *chatCache) DeleteCachedMessages(conversationID string) error {
	messagesKey := fmt.Sprintf(readCachedMessageStr, conversationID)
	cachedMessages, _ := c.redisStore.Read(messagesKey)
	for _, m := range cachedMessages {
		if err := c.redisStore.Delete(m.Key); err != nil {
			return err
		}
	}
	return nil
}

func (c *chatCache) DeleteCachedMessage(key string) error {
	return c.redisStore.Delete(key)
}
