package notifications

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Channel is the Postgres LISTEN/NOTIFY channel that db/queries/notifications.sql publishes to.
const Channel = "classlab_notifications"

const (
	subscriberBuffer = 32
	reconnectDelay   = 2 * time.Second
)

// Resync is sent to subscribers after the listener lost its connection and came back: notifications
// that arrived in the gap were not delivered, so the client should refetch its list.
var Resync = uuid.Nil

// Hub fans notifications out from Postgres to the open streams of this process.
//
// One dedicated connection LISTENs on Channel. Postgres delivers a NOTIFY only when the sending
// transaction commits, so a stream hears about a notification exactly when it becomes visible and
// never about one that rolled back. The hub does not use the pool: a LISTEN connection must stay
// checked out for the life of the process, which would make pool.Close wait forever.
type Hub struct {
	ctx        context.Context // bounds the listener; cancelled on server shutdown
	connString string
	log        *slog.Logger

	startOnce sync.Once
	readyOnce sync.Once
	ready     chan struct{} // closed once the first LISTEN is in place

	mu   sync.Mutex
	subs map[uuid.UUID]map[*Subscription]struct{}
}

type Subscription struct {
	// C carries notification ids for one user, or Resync. It is closed when the subscription ends,
	// including when the subscriber fell too far behind to keep up.
	C <-chan uuid.UUID

	c    chan uuid.UUID
	user uuid.UUID
	hub  *Hub
}

// NewHub does not connect; the listener starts on the first Subscribe, so a server nobody streams
// from holds no extra connection. ctx should live as long as the server.
func NewHub(ctx context.Context, connString string, log *slog.Logger) *Hub {
	return &Hub{
		ctx: ctx, connString: connString, log: log,
		ready: make(chan struct{}),
		subs:  make(map[uuid.UUID]map[*Subscription]struct{}),
	}
}

// Subscribe registers for a user's notifications and starts the listener on first use.
func (h *Hub) Subscribe(user uuid.UUID) *Subscription {
	h.startOnce.Do(func() { go h.run() })

	c := make(chan uuid.UUID, subscriberBuffer)
	s := &Subscription{C: c, c: c, user: user, hub: h}
	h.mu.Lock()
	if h.subs[user] == nil {
		h.subs[user] = make(map[*Subscription]struct{})
	}
	h.subs[user][s] = struct{}{}
	h.mu.Unlock()
	return s
}

// Ready is closed once the hub is listening. A subscriber that must not miss anything waits on it
// before announcing itself to the client.
func (h *Hub) Ready() <-chan struct{} { return h.ready }

func (s *Subscription) Close() {
	s.hub.mu.Lock()
	defer s.hub.mu.Unlock()
	s.hub.removeLocked(s)
}

func (h *Hub) removeLocked(s *Subscription) {
	set := h.subs[s.user]
	if _, ok := set[s]; !ok {
		return
	}
	delete(set, s)
	close(s.c)
	if len(set) == 0 {
		delete(h.subs, s.user)
	}
}

func (h *Hub) run() {
	listened := false
	for h.ctx.Err() == nil {
		err := h.listen(&listened)
		if h.ctx.Err() != nil {
			return
		}
		h.log.Warn("notification listener lost its connection; retrying", slog.String("error", err.Error()))
		select {
		case <-h.ctx.Done():
			return
		case <-time.After(reconnectDelay):
		}
	}
}

// listen holds one connection until it fails or the hub's context ends. listened records whether a
// LISTEN has ever succeeded, which tells a first start from a recovery.
func (h *Hub) listen(listened *bool) error {
	conn, err := pgx.Connect(h.ctx, h.connString)
	if err != nil {
		return err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(h.ctx), 2*time.Second)
		defer cancel()
		if err := conn.Close(closeCtx); err != nil {
			h.log.Debug("closing notification listener connection", slog.String("error", err.Error()))
		}
	}()

	if _, err := conn.Exec(h.ctx, "LISTEN "+Channel); err != nil {
		return err
	}
	h.readyOnce.Do(func() { close(h.ready) })
	if *listened {
		h.broadcast(Resync)
	}
	*listened = true

	for {
		n, err := conn.WaitForNotification(h.ctx)
		if err != nil {
			return err
		}
		user, id, ok := parsePayload(n.Payload)
		if !ok {
			h.log.Warn("ignoring malformed notification payload")
			continue
		}
		h.deliver(user, id)
	}
}

// parsePayload reads "<user_id>:<notification_id>".
func parsePayload(p string) (user, id uuid.UUID, ok bool) {
	u, n, found := strings.Cut(p, ":")
	if !found {
		return uuid.Nil, uuid.Nil, false
	}
	user, errU := uuid.Parse(u)
	id, errN := uuid.Parse(n)
	return user, id, errU == nil && errN == nil
}

func (h *Hub) deliver(user, id uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs[user] {
		h.sendLocked(s, id)
	}
}

func (h *Hub) broadcast(id uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, set := range h.subs {
		for s := range set {
			h.sendLocked(s, id)
		}
	}
}

// sendLocked never blocks: a subscriber whose buffer is full is dropped, and its client reconnects
// and refetches. Slowness in one stream must not hold up notifications for everyone else. Sending
// and closing both happen under h.mu, so a send can never hit a closed channel.
func (h *Hub) sendLocked(s *Subscription, id uuid.UUID) {
	select {
	case s.c <- id:
	default:
		h.removeLocked(s)
	}
}

// Done is closed when the hub's context ends, i.e. the server is shutting down.
func (h *Hub) Done() <-chan struct{} { return h.ctx.Done() }
