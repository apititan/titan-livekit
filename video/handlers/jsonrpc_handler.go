package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pion/ion-sfu/cmd/signal/json-rpc/server"
	"github.com/sourcegraph/jsonrpc2"
)

type JsonRpcHandler struct {
	*server.JSONSignal
	httpHandler *HttpHandler
}

type ContextData struct {
	userId int64
	chatId int64
}

// key is an unexported type for keys defined in this package.
// This prevents collisions with keys defined in other packages.
type key int

// contextDataKey is the key for user.User values in Contexts. It is
// unexported; clients use user.NewContext and user.FromContext
// instead of using this key directly.
var contextDataKey key

// NewContext returns a new Context that carries value u.
func NewContext(ctx context.Context, u *ContextData) context.Context {
	return context.WithValue(ctx, contextDataKey, u)
}

// FromContext returns the User value stored in ctx, if any.
func FromContext(ctx context.Context) (*ContextData, bool) {
	u, ok := ctx.Value(contextDataKey).(*ContextData)
	return u, ok
}

type UserByStreamId struct {
	StreamId string `json:"streamId"`
}


func (p *JsonRpcHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	replyError := func(err error) {
		_ = conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
			Code:    500,
			Message: fmt.Sprintf("%s", err),
		})
	}

	fromContext, b := FromContext(ctx)
	if !b {
		err := errors.New("unable to extract data from context")
		p.Logger.Error(err, "problem with getting tata from context")
		replyError(err)

	}

	switch req.Method {
	case "userByStreamId":
		var userByStreamId UserByStreamId
		err := json.Unmarshal(*req.Params, &userByStreamId)
		if err != nil {
			p.Logger.Error(err, "error parsing UserByStreamId request")
			replyError(err)
			break
		}
		p.httpHandler.getPeerMetadataByStreamId()
		fromContext.chatId, fromContext.userId, userByStreamId.StreamId
	}
}

func (h *JsonRpcHandler) userByStreamId() {

}