// Package push owns the VAPID key pair, the push subscriptions and the sending of Web Push
// notifications (design doc item 57, "Daily push").
package push

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/jnaranj09/open-customer/internal/db"
	"github.com/jnaranj09/open-customer/internal/db/sqlc"
	"github.com/jnaranj09/open-customer/internal/platform/clock"
	"github.com/jnaranj09/open-customer/internal/platform/ids"
	"github.com/jnaranj09/open-customer/internal/platform/problem"
)

const (
	keyPublic  = "vapid_public"
	keyPrivate = "vapid_private"

	// ttlSeconds keeps an undelivered push for 8 hours: the same day, no later.
	ttlSeconds = 28800
	// defaultTimeout bounds one send when Options.HTTPClient is nil.
	defaultTimeout = 15 * time.Second
	// jwtLifetime is the VAPID JWT expiry the library defaults to.
	jwtLifetime = 12 * time.Hour

	p256dhLen = 65 // an uncompressed P-256 point
	authLen   = 16
)

// Options is what serve passes to New.
type Options struct {
	Clock clock.Clock
	// Subject is VAPID_SUBJECT, a "mailto:" address.
	Subject string
	// HTTPClient sends to the push services; nil means publicOnlyClient (a 15 s timeout, public
	// addresses only). Either way the module's copy follows no redirect.
	HTTPClient *http.Client
	// Resolver looks up an endpoint's host when it is subscribed; nil means net.DefaultResolver.
	Resolver Resolver
}

// Resolver is the DNS lookup Subscribe uses; *net.Resolver has it.
type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// Module is the push module. Every method takes ctx and a db.DBTX first.
type Module struct {
	clock clock.Clock
	// subscriber is Subject without "mailto:": webpush-go v1.4.0 adds "mailto:" to any subscriber
	// that does not start with "https:", and the JWT sub must have exactly one.
	subscriber string
	client     *http.Client
	resolver   Resolver
}

// New returns the push module.
func New(opts Options) *Module {
	client := publicOnlyClient()
	if opts.HTTPClient != nil {
		client = new(*opts.HTTPClient)
	}
	client.CheckRedirect = refuseRedirect
	var resolver Resolver = net.DefaultResolver
	if opts.Resolver != nil {
		resolver = opts.Resolver
	}
	return &Module{clock: opts.Clock, subscriber: strings.TrimPrefix(opts.Subject, "mailto:"), client: client, resolver: resolver}
}

// Subscription is the browser's PushSubscription: the endpoint and its keys, base64url.
type Subscription struct{ Endpoint, P256dh, Auth string }

// Payload is the JSON the service worker gets (design doc "Push payload").
type Payload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Count int    `json:"count"`
	URL   string `json:"url"`
}

// SendResult is how many subscriptions SendAll tried and how many the push services accepted.
type SendResult struct{ Attempted, Delivered int }

// EnsureKeys generates the VAPID key pair once and stores it in server_keys. When the public key
// is already there it changes nothing. Run it in a transaction, so the two keys are stored
// together.
func (m *Module) EnsureKeys(ctx context.Context, q db.DBTX) error {
	queries := sqlc.New(q)
	_, err := queries.GetServerKey(ctx, keyPublic)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("get vapid public key: %w", err)
	}
	private, public, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return fmt.Errorf("generate vapid keys: %w", err)
	}
	now := m.clock.Now()
	for _, k := range []sqlc.InsertServerKeyIfMissingParams{
		{Name: keyPrivate, Value: private, Now: now},
		{Name: keyPublic, Value: public, Now: now},
	} {
		if err := queries.InsertServerKeyIfMissing(ctx, k); err != nil {
			return fmt.Errorf("insert %s: %w", k.Name, err)
		}
	}
	return nil
}

// PublicKey returns the VAPID public key, base64url without padding.
func (m *Module) PublicKey(ctx context.Context, q db.DBTX) (string, error) {
	key, err := sqlc.New(q).GetServerKey(ctx, keyPublic)
	if err != nil {
		return "", fmt.Errorf("get vapid public key: %w", err)
	}
	return key, nil
}

// Checked is a Subscription that Check accepted. Only Check makes a non-zero one, so Subscribe
// never stores an unchecked endpoint.
type Checked struct{ sub Subscription }

// errUnchecked is a zero Checked passed to Subscribe: a caller bug, not bad input.
var errUnchecked = errors.New("push subscription was not checked")

// Check is the one parse site of a browser's subscription. A bad field is a problem.Validation on
// endpoint, keys.p256dh or keys.auth. It looks the endpoint's host up in DNS, so the handler calls
// it before it opens the write transaction; it takes no database.
func (m *Module) Check(ctx context.Context, s Subscription) (Checked, error) {
	if err := m.checkSubscription(ctx, s); err != nil {
		return Checked{}, err
	}
	return Checked{sub: s}, nil
}

// Subscribe stores c for userID, or gives a known endpoint the new keys (its row and ID stay).
// created reports a new row.
func (m *Module) Subscribe(ctx context.Context, q db.DBTX, userID uuid.UUID, c Checked) (uuid.UUID, bool, error) {
	if c.sub.Endpoint == "" {
		return uuid.Nil, false, errUnchecked
	}
	s := c.sub
	id, err := ids.New()
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("new subscription id: %w", err)
	}
	row, err := sqlc.New(q).UpsertSubscription(ctx, sqlc.UpsertSubscriptionParams{
		ID: id, UserID: userID, Endpoint: s.Endpoint, P256dh: s.P256dh, Auth: s.Auth, Now: m.clock.Now(),
	})
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("upsert subscription: %w", err)
	}
	return row.ID, row.Created, nil
}

// checkSubscription reports every bad field: the endpoint must be an absolute https URL whose host
// is a name with a public address (publicHost), p256dh must decode to 65 bytes and auth to 16.
func (m *Module) checkSubscription(ctx context.Context, s Subscription) error {
	var errs []problem.FieldError
	if u, err := url.Parse(s.Endpoint); err != nil || u.Scheme != "https" || u.Host == "" || !publicHost(ctx, m.resolver, u) {
		errs = append(errs, problem.FieldError{Field: "endpoint", Message: problem.MsgPushEndpointInvalid})
	}
	if decodedLen(s.P256dh) != p256dhLen {
		errs = append(errs, problem.FieldError{Field: "keys.p256dh", Message: problem.MsgPushEndpointInvalid})
	}
	if decodedLen(s.Auth) != authLen {
		errs = append(errs, problem.FieldError{Field: "keys.auth", Message: problem.MsgPushEndpointInvalid})
	}
	if len(errs) > 0 {
		return problem.Validation{Errors: errs}
	}
	return nil
}

// decodedLen is the length of base64url key (padded or not), or -1 when it is not base64url.
func decodedLen(key string) int {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(key, "="))
	if err != nil {
		return -1
	}
	return len(raw)
}

// CountSubscriptions returns how many push subscriptions are stored.
func (m *Module) CountSubscriptions(ctx context.Context, q db.DBTX) (int, error) {
	n, err := sqlc.New(q).CountSubscriptions(ctx)
	if err != nil {
		return 0, fmt.Errorf("count subscriptions: %w", err)
	}
	return int(n), nil
}

// SendAll sends p to every subscription, one after another. A 2xx answer counts as delivered. A
// 404 or 410 deletes the subscription. Any other answer, or a network error, counts as not
// delivered and logs code=push_send_failed with the status only: the push services are not ours
// and never fail the caller. Database errors are returned.
func (m *Module) SendAll(ctx context.Context, q db.DBTX, p Payload) (SendResult, error) {
	queries := sqlc.New(q)
	subs, err := queries.ListSubscriptions(ctx)
	if err != nil {
		return SendResult{}, fmt.Errorf("list subscriptions: %w", err)
	}
	if len(subs) == 0 {
		return SendResult{}, nil
	}
	public, err := queries.GetServerKey(ctx, keyPublic)
	if err != nil {
		return SendResult{}, fmt.Errorf("get vapid public key: %w", err)
	}
	private, err := queries.GetServerKey(ctx, keyPrivate)
	if err != nil {
		return SendResult{}, fmt.Errorf("get vapid private key: %w", err)
	}
	message, err := json.Marshal(p)
	if err != nil {
		return SendResult{}, fmt.Errorf("encode payload: %w", err)
	}
	opts := &webpush.Options{
		HTTPClient:      m.client,
		Subscriber:      m.subscriber,
		TTL:             ttlSeconds,
		Urgency:         webpush.UrgencyNormal,
		VAPIDPublicKey:  public,
		VAPIDPrivateKey: private,
		// v1.4.0 signs its JWT with its own clock and ignores this; it is set so the module's
		// time still comes from the injected clock.
		VapidExpiration: m.clock.Now().Add(jwtLifetime),
	}
	var res SendResult
	for _, s := range subs {
		res.Attempted++
		sub := &webpush.Subscription{Endpoint: s.Endpoint, Keys: webpush.Keys{P256dh: s.P256dh, Auth: s.Auth}}
		status, sent := send(ctx, message, sub, opts)
		switch {
		case sent && status >= 200 && status < 300:
			res.Delivered++
		case sent && (status == http.StatusNotFound || status == http.StatusGone):
			if err := queries.DeleteSubscription(ctx, s.ID); err != nil {
				return res, fmt.Errorf("delete gone subscription: %w", err)
			}
		case sent:
			slog.WarnContext(ctx, "push service did not accept the notification", "code", "push_send_failed", "status", status)
		default:
			// Never log the error: it quotes the endpoint.
			slog.WarnContext(ctx, "push notification could not be sent", "code", "push_send_failed")
		}
	}
	return res, nil
}

// send makes one request and closes its body. sent is false when no answer came back.
func send(ctx context.Context, message []byte, sub *webpush.Subscription, opts *webpush.Options) (status int, sent bool) {
	resp, err := webpush.SendNotificationWithContext(ctx, message, sub, opts)
	if err != nil {
		return 0, false
	}
	// The body is unused; it is closed unread.
	if err := resp.Body.Close(); err != nil {
		slog.WarnContext(ctx, "push answer body could not be closed", "code", "push_body_close_failed")
	}
	return resp.StatusCode, true
}
