package fixtures

import (
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
)

type input struct{ Name string }

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) error { return nil }

func decodePatch(w http.ResponseWriter, r *http.Request, v any, nullable ...string) error {
	return nil
}

type server struct{ log *slog.Logger }

// The request body itself.
func logsTheBody(w http.ResponseWriter, r *http.Request) {
	// ruleid: no-slog-of-request-body
	slog.Warn("bad request", "body", r.Body)
	// ruleid: no-slog-of-request-body
	slog.WarnContext(r.Context(), "bad request", "form", r.PostForm)
}

// The decoded value, or a field of it.
func logsTheDecodedValue(w http.ResponseWriter, r *http.Request) {
	var body input
	if decodeJSON(w, r, &body) != nil {
		return
	}
	// ruleid: no-slog-of-request-body
	slog.Info("created", "input", body)
	// ruleid: no-slog-of-request-body
	slog.Info("created", "name", body.Name)
}

// The error decodeJSON returns, which can quote the input.
func logsTheDecodeError(w http.ResponseWriter, r *http.Request, s *server) {
	var body input
	err := decodeJSON(w, r, &body)
	if err != nil {
		// ruleid: no-slog-of-request-body
		s.log.ErrorContext(r.Context(), "decode failed", "err", err)
		// ruleid: no-slog-of-request-body
		slog.Error("decode failed", slog.Any("err", err))
	}
	perr := decodePatch(w, r, &body, "name")
	if perr != nil {
		// ruleid: no-slog-of-request-body
		slog.Error("patch failed", "err", perr)
	}
}

// A path and a code, never the value.
func logsACode(w http.ResponseWriter, r *http.Request, s *server) {
	var body input
	if decodeJSON(w, r, &body) != nil {
		// ok: no-slog-of-request-body
		slog.WarnContext(r.Context(), "body rejected", "code", "body_invalid", "path", r.URL.Path)
		return
	}
	// ok: no-slog-of-request-body
	s.log.Info("created", "code", "client_created")
	// ok: no-slog-of-request-body
	slog.Info("len", "bytes", r.ContentLength)
}

// A log of something other than the request is not the rule's business.
func logsAnotherError(r *http.Request) {
	err := http.ErrAbortHandler
	// ok: no-slog-of-request-body
	slog.Error("aborted", "err", err)
}

// A body read with the standard decoder, and the standard logger.
func logsAStdDecodedBody(w http.ResponseWriter, r *http.Request) {
	var body input
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		// ruleid: no-slog-of-request-body
		slog.Warn("bad", "err", err)
		return
	}
	// ruleid: no-slog-of-request-body
	slog.Info("created", "input", body)
	// ruleid: no-slog-of-request-body
	log.Printf("created %v", body)
	// ruleid: no-slog-of-request-body
	log.Println(r.Body)
	// ok: no-slog-of-request-body
	log.Printf("created %s", "client")
}

func logsTheDecodeErrorWithStdLog(w http.ResponseWriter, r *http.Request) {
	var body input
	err := decodeJSON(w, r, &body)
	if err != nil {
		// ruleid: no-slog-of-request-body
		log.Fatalf("decode: %v", err)
	}
}

// The error variable is reused: after another call gives it a value it is not the decode error.
func reusesTheErrorVariable(w http.ResponseWriter, r *http.Request) {
	var body input
	err := decodeJSON(w, r, &body)
	if err != nil {
		// ruleid: no-slog-of-request-body
		slog.Warn("decode", "err", err)
		return
	}
	_, err = http.Get("http://made-up.example")
	if err != nil {
		// ok: no-slog-of-request-body
		slog.Warn("get", "err", err)
	}
	if err := http.ErrAbortHandler; err != nil {
		// ok: no-slog-of-request-body
		slog.Warn("abort", "err", err)
	}
	res, err := http.Get("http://made-up.example")
	if err != nil {
		// ok: no-slog-of-request-body
		slog.Warn("get", "err", err, "res", res)
	}
}
