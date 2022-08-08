package client

import (
	"context"
	"reflect"

	"github.com/gotd/td/telegram/peers"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"github.com/micro/micro/v3/service/errors"
	"github.com/webitel/chat_manager/api/proto/chat"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Broadcast given `req.Message` message [to] provided `req.Peer(s)`
func (c *App) BroadcastMessage(ctx context.Context, req *chat.BroadcastMessageRequest, rsp *chat.BroadcastMessageResponse) error {

	if c.me == nil {
		return errors.BadGateway(
			"chat.broadcast.telegram.unauthorized",
			"telegram: app unauthorized",
		)
	}

	var (
		n          = len(req.GetPeer())
		inputPeers = make([]struct {
			id    int
			input tg.InputPeerClass
		}, 0, n)
		resolvedPeer = func(peerId int, peer tg.InputPeerClass) error {
			if peer.Zero() {
				return &peers.PhoneNotFoundError{}
			}
			for _, resolved := range inputPeers {
				if reflect.DeepEqual(resolved.input, peer) {
					return errors.BadRequest(
						"chat.broadcast.peer.duplicate",
						"peer: duplicate; ignore",
					)
				}
			}
			// inputPeers[id] = peer
			inputPeers = append(inputPeers, struct {
				id    int
				input tg.InputPeerClass
			}{
				id:    peerId,
				input: peer,
			})

			return nil
		}
		resolvedError = func(peerId int, err error) {

			res := rsp.GetFailure()
			if res == nil {
				res = make([]*chat.BroadcastPeer, 0, n)
			}

			var re *status.Status
			switch err := err.(type) {
			case *tgerr.Error:
				re = status.New(codes.Code(err.Code), err.Message)
			case *errors.Error:
				re = status.New(codes.Code(err.Code), err.Detail)
			default:
				re = status.New(codes.Unknown, err.Error())
			}

			res = append(res, &chat.BroadcastPeer{
				Peer:  req.Peer[peerId],
				Error: re.Proto(),
			})

			rsp.Failure = res
		}
		id int
	)

	for id < n {
		peerId := req.Peer[id]
		peer, err := c.resolve(ctx, peerId)
		if flood, err := tgerr.FloodWait(ctx, err); err != nil {
			if flood || tgerr.Is(err, tg.ErrTimeout) {
				continue // retry
			}
			resolvedError(id, err)
			id++ // next
			continue
		}
		err = resolvedPeer(id, peer.InputPeer())
		if err != nil {
			resolvedError(id, err)
		}
		id++ // next
		// continue
	}

	// PERFORM: sendMessage to resolved peer(s)...
	id, n = 0, len(inputPeers)
	for id < n {
		peer := inputPeers[id].input
		sendMessage := c.Sender.To(peer)
		// TODO: transform given msg to sendMessage
		// sendMessage.Text(ctx, "message test")
		_, err := sendMessage.Text(ctx, req.GetMessage().GetText())
		if flood, err := tgerr.FloodWait(ctx, err); err != nil {
			if flood || tgerr.Is(err, tg.ErrTimeout) {
				continue // retry
			}
			resolvedError(inputPeers[id].id, err)
			id++ // next
			continue
		}
		id++ // next
		// continue
	}

	return nil
}
