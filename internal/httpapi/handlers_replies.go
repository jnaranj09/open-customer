package httpapi

import (
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/jnaranj09/open-customer/internal/httpapi/gen"
	"github.com/jnaranj09/open-customer/internal/replies"
)

// ListQuickReplies answers every quick reply, archived ones too, by position.
func (s *Server) ListQuickReplies(w http.ResponseWriter, r *http.Request) {
	list, err := s.replies.List(r.Context(), s.pool)
	if err != nil {
		writeError(w, r, fmt.Errorf("list quick replies: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, gen.QuickReplyList{Items: toGenReplies(list)})
}

// CreateQuickReply adds a reply; it goes last.
func (s *Server) CreateQuickReply(w http.ResponseWriter, r *http.Request) {
	var body gen.QuickReplyInput
	if decodeJSON(w, r, &body) != nil {
		return
	}
	in := replies.ReplyInput{Title: body.Title, Body: body.Body}
	s.replyTx(w, r, http.StatusCreated, func(tx pgx.Tx) (replies.Reply, error) {
		return s.replies.Create(r.Context(), tx, in)
	})
}

// UpdateQuickReply changes the fields sent; none may be null.
func (s *Server) UpdateQuickReply(w http.ResponseWriter, r *http.Request, id openapi_types.UUID) {
	var body gen.QuickReplyPatch
	if decodePatch(w, r, &body) != nil {
		return
	}
	patch := replies.ReplyPatch{Title: body.Title, Body: body.Body, Position: body.Position, Archived: body.Archived}
	s.replyTx(w, r, http.StatusOK, func(tx pgx.Tx) (replies.Reply, error) {
		return s.replies.Update(r.Context(), tx, id, patch)
	})
}

// replyTx runs one replies write in the write transaction and answers the reply with status.
func (s *Server) replyTx(w http.ResponseWriter, r *http.Request, status int, call func(pgx.Tx) (replies.Reply, error)) {
	var out replies.Reply
	if !s.inWriteTx(w, r, func(tx pgx.Tx) error {
		var err error
		if out, err = call(tx); err != nil {
			return fmt.Errorf("quick reply: %w", err)
		}
		return nil
	}) {
		return
	}
	writeJSON(w, status, toGenReply(out))
}

func toGenReply(v replies.Reply) gen.QuickReply {
	return gen.QuickReply{Id: v.ID, Title: v.Title, Body: v.Body, Position: v.Position, Archived: v.Archived}
}

// toGenReplies is the wire list of replies; never nil.
func toGenReplies(list []replies.Reply) []gen.QuickReply {
	out := make([]gen.QuickReply, 0, len(list))
	for _, v := range list {
		out = append(out, toGenReply(v))
	}
	return out
}
