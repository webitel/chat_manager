package chat

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/micro/micro/v3/service/broker"
	"github.com/micro/micro/v3/service/broker/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbchat "github.com/webitel/chat_manager/api/proto/chat"
	"github.com/webitel/chat_manager/app"
	event "github.com/webitel/chat_manager/internal/event_router"
	"github.com/webitel/chat_manager/internal/flow"
	pg "github.com/webitel/chat_manager/internal/repo/sqlx"
)

// closeLeaveDB is an in-memory chat.channel store, that mimics
// the `closed_at ISNULL` guards of the real close queries.
// NOT implemented pg.Repository methods panic.
type closeLeaveDB struct {
	pg.Repository

	mx       sync.Mutex
	channels []*pg.Channel
	causes   map[string]string // [channel.id] closed_cause
}

func (db *closeLeaveDB) get(id string) *pg.Channel {
	for _, c := range db.channels {
		if c.ID == id {
			return c
		}
	}
	return nil
}

func (db *closeLeaveDB) close(c *pg.Channel, cause string) {
	c.ClosedAt = sql.NullTime{Time: time.Now(), Valid: true}
	db.causes[c.ID] = cause
}

func (db *closeLeaveDB) cause(id string) string {
	db.mx.Lock()
	defer db.mx.Unlock()
	return db.causes[id]
}

func appChannel(c *pg.Channel) *app.Channel {
	member := &app.Channel{
		Chat: &app.Chat{
			ID:      c.ID,
			Channel: c.Type,
			Invite:  c.ConversationID,
		},
		User: &app.User{
			ID:      c.UserID,
			Channel: c.Type,
		},
		DomainID: c.DomainID,
	}
	if c.ClosedAt.Valid {
		member.Closed = c.ClosedAt.Time.Unix()
	}
	return member
}

func (db *closeLeaveDB) GetSession(_ context.Context, chatID string) (*app.Session, error) {
	db.mx.Lock()
	defer db.mx.Unlock()
	sender := db.get(chatID)
	if sender == nil {
		return nil, nil
	}
	chat := &app.Session{Channel: appChannel(sender)}
	for _, c := range db.channels {
		if c != sender && c.ConversationID == sender.ConversationID {
			chat.Members = append(chat.Members, appChannel(c))
		}
	}
	return chat, nil
}

func (db *closeLeaveDB) CheckUserChannel(_ context.Context, id string, userID int64) (*pg.Channel, error) {
	db.mx.Lock()
	defer db.mx.Unlock()
	c := db.get(id)
	if c == nil || c.UserID != userID {
		return nil, nil
	}
	found := *c
	return &found, nil
}

func (db *closeLeaveDB) CloseChannel(_ context.Context, id string, cause string) (*pg.Channel, error) {
	db.mx.Lock()
	defer db.mx.Unlock()
	c := db.get(id)
	if c == nil || c.ClosedAt.Valid {
		return nil, nil // NOT FOUND -or- already CLOSED
	}
	db.close(c, cause)
	closed := *c
	return &closed, nil
}

func (db *closeLeaveDB) CloseConversation(_ context.Context, id string, cause string) error {
	db.mx.Lock()
	defer db.mx.Unlock()
	for _, c := range db.channels {
		if c.ConversationID == id && !c.ClosedAt.Valid {
			db.close(c, cause)
		}
	}
	return nil
}

// closeLeaveFlow plays the chat@workflow service
type closeLeaveFlow struct {
	flow.Client
	// onBreak is the call_center reaction to the chat@workflow .Break()
	onBreak func(conversationID string)
}

func (f *closeLeaveFlow) CloseConversation(conversationID, _ string) error {
	if f.onBreak != nil {
		f.onBreak(conversationID)
	}
	return nil
}

func (f *closeLeaveFlow) BreakBridge(string, flow.BreakBridgeCause) error {
	return nil
}

// closeLeaveRouter records events, sent to the chat members
type closeLeaveRouter struct {
	event.Router

	mx     sync.Mutex
	left   []string // [channel.id] announced as left
	closed []string // [channel.id] gateway "closed" notices
}

func (r *closeLeaveRouter) RouteLeaveConversation(c *pg.Channel, _ *string, _ string) error {
	r.mx.Lock()
	defer r.mx.Unlock()
	r.left = append(r.left, c.ID)
	return nil
}

func (r *closeLeaveRouter) SendMessageToGateway(_, target *app.Channel, message *pbchat.Message) error {
	r.mx.Lock()
	defer r.mx.Unlock()
	if message.GetType() == "closed" {
		r.closed = append(r.closed, target.ID)
	}
	return nil
}

const (
	testConversationID = "conv-1"
	testClientChatID   = "client-1"
	testClientUserID   = 10
	testAgentChatID    = "agent-1"
	testAgentUserID    = 20
)

// newCloseLeaveService returns the chat service with
// the client, chat@workflow and agent channels bridged in a single conversation
func newCloseLeaveService(t *testing.T) (*chatService, *closeLeaveDB, *closeLeaveFlow, *closeLeaveRouter) {
	t.Helper()

	// websocket (agent) members are notified via the broker
	agents := memory.NewBroker()
	require.NoError(t, agents.Connect())
	defaultBroker := broker.DefaultBroker
	broker.DefaultBroker = agents
	t.Cleanup(func() {
		broker.DefaultBroker = defaultBroker
		_ = agents.Disconnect()
	})

	db := &closeLeaveDB{
		channels: []*pg.Channel{
			{ID: testClientChatID, Type: "webchat", ConversationID: testConversationID, UserID: testClientUserID, DomainID: 1},
			{ID: testConversationID, Type: "chatflow", ConversationID: testConversationID, UserID: 1, DomainID: 1, Internal: true},
			{ID: testAgentChatID, Type: "websocket", ConversationID: testConversationID, UserID: testAgentUserID, DomainID: 1, Internal: true},
		},
		causes: make(map[string]string),
	}
	workflow := &closeLeaveFlow{}
	router := &closeLeaveRouter{}

	srv := NewChatService(
		db, slog.New(slog.NewTextHandler(io.Discard, nil)),
		workflow, nil, nil, nil, router,
	).(*chatService)

	return srv, db, workflow, router
}

// step is a single chat service request in the test scenario
type step func(srv *chatService) error

// agentLeave performs .LeaveConversation() on behalf of call_center
func agentLeave(cause pbchat.LeaveConversationCause) step {
	return func(srv *chatService) error {
		return srv.LeaveConversation(
			context.Background(),
			&pbchat.LeaveConversationRequest{
				ConversationId: testConversationID,
				ChannelId:      testAgentChatID,
				AuthUserId:     testAgentUserID,
				Cause:          cause,
			},
			&pbchat.LeaveConversationResponse{},
		)
	}
}

// clientClose performs .CloseConversation() on behalf of the client (/close)
func clientClose(srv *chatService) error {
	return srv.CloseConversation(
		context.Background(),
		&pbchat.CloseConversationRequest{
			ConversationId:  testConversationID,
			CloserChannelId: testClientChatID,
			AuthUserId:      testClientUserID,
			Cause:           pbchat.CloseConversationCause_client_leave,
		},
		&pbchat.CloseConversationResponse{},
	)
}

// flowClose performs .CloseConversation() on behalf of the chat@workflow (flow_end)
func flowClose(srv *chatService) error {
	return srv.CloseConversation(
		context.Background(),
		&pbchat.CloseConversationRequest{
			ConversationId:  testConversationID,
			CloserChannelId: testConversationID,
			Cause:           pbchat.CloseConversationCause_flow_end,
		},
		&pbchat.CloseConversationResponse{},
	)
}

// WTEL-10713: the chat@workflow .Break() makes call_center cancel the queue attempt
// and leave the agent channel before CloseConversation() marks channels closed.
//
// NOTE: NOT parallel ! Each case replaces the global broker.DefaultBroker
func TestCloseConversation_ConcurrentLeave(t *testing.T) {
	tests := []struct {
		name string
		// onBreak is performed on the chat@workflow .Break(), when not nil
		onBreak step
		steps   []step

		wantCause  string   // agent channel closed_cause
		wantLeft   []string // leave_conversation recipients
		wantClosed []string // gateway "closed" notice recipients
	}{
		{
			name:       "agent leave on flow break keeps close cause",
			onBreak:    agentLeave(pbchat.LeaveConversationCause_agent_leave),
			steps:      []step{clientClose},
			wantCause:  "client_leave",
			wantClosed: []string{testClientChatID},
		},
		{
			name:       "flow close on flow break notifies once",
			onBreak:    flowClose,
			steps:      []step{clientClose},
			wantCause:  "client_leave",
			wantClosed: []string{testClientChatID},
		},
		{
			name:       "agent leave after close is noop",
			steps:      []step{clientClose, agentLeave(pbchat.LeaveConversationCause_agent_leave)},
			wantCause:  "client_leave",
			wantClosed: []string{testClientChatID},
		},
		{
			name:      "agent leave without close keeps leave cause",
			steps:     []step{agentLeave(pbchat.LeaveConversationCause_agent_timeout)},
			wantCause: "agent_timeout",
			wantLeft:  []string{testAgentChatID},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, db, workflow, router := newCloseLeaveService(t)

			var breakErr error
			if tc.onBreak != nil {
				workflow.onBreak = func(string) {
					breakErr = tc.onBreak(srv)
				}
			}

			for _, perform := range tc.steps {
				require.NoError(t, perform(srv))
			}
			require.NoError(t, breakErr)

			assert.Equal(t, tc.wantCause, db.cause(testAgentChatID), "agent channel closed_cause")
			assert.Equal(t, tc.wantLeft, router.left, "leave_conversation recipients")
			assert.Equal(t, tc.wantClosed, router.closed, "gateway closed notice recipients")
		})
	}
}
