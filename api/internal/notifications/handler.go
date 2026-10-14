package notifications

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"classlab/api/internal/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// StreamOptions tunes GET /notifications/stream.
type StreamOptions struct {
	// Heartbeat is how often a comment line is sent so proxies and load balancers keep the
	// connection open.
	Heartbeat time.Duration
	// MaxLifetime ends a stream so that it never outlives the access token it was opened with. The
	// client then reconnects with a fresh token, which also re-checks that the account is still active.
	MaxLifetime time.Duration
	// MaxPerUser caps the streams one user may keep open at once (a few tabs and devices are normal).
	MaxPerUser int
}

type Handler struct {
	svc  *Service
	hub  *Hub
	opts StreamOptions
	log  *slog.Logger
}

func NewHandler(svc *Service, hub *Hub, opts StreamOptions, log *slog.Logger) *Handler {
	return &Handler{svc: svc, hub: hub, opts: opts, log: log}
}

// Register mounts the routes on a router that is already behind httpx.Authenticate. Every role has
// notifications, and each user sees only their own.
func (h *Handler) Register(r chi.Router) {
	r.Get("/", h.list)
	r.Get("/stream", h.stream)
	r.Post("/read-all", h.readAll)
	r.Post("/{id}/read", h.markRead)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())

	page, perPage, err := httpx.ParsePage(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	unread := false
	if v := r.URL.Query().Get("unread"); v != "" {
		unread, err = strconv.ParseBool(v)
		if err != nil {
			httpx.WriteError(w, r, httpx.ValidationError(httpx.ErrorDetail{Field: "unread", Message: "unread harus true atau false."}))
			return
		}
	}

	items, total, err := h.svc.List(r.Context(), actor, unread, page, perPage)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.ListResponse[Notification]{
		Data: items, Meta: httpx.Meta{Page: page, PerPage: perPage, Total: total},
	})
}

func (h *Handler) markRead(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound)
		return
	}
	n, err := h.svc.MarkRead(r.Context(), actor, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, n)
}

func (h *Handler) readAll(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())
	n, err := h.svc.MarkAllRead(r.Context(), actor)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]int64{"updated": n})
}

// stream serves Server-Sent Events:
//
//	event: ready         {"unread": N}   the stream is live; sent once, after subscribing
//	event: notification  <Notification>  a new notification for this user (id: is its UUID)
//	event: resync        {}              the server lost events; refetch GET /notifications
//	event: reconnect     {}              the server is ending the stream; open a new one
//
// The browser EventSource cannot send an Authorization header, so clients read the stream with fetch.
// Events are not replayed: after any (re)connect the client should list notifications itself.
func (h *Handler) stream(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.PrincipalFrom(r.Context())
	rc := http.NewResponseController(w)

	// The server's WriteTimeout would cut the stream off; a stream needs its own lifetime.
	if err := rc.SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
		h.log.WarnContext(r.Context(), "clearing write deadline for notification stream", slog.String("error", err.Error()))
	}

	// Subscribe before counting, so a notification created in between is not missed by both.
	sub, ok := h.hub.Subscribe(actor.UserID, h.opts.MaxPerUser)
	if !ok {
		w.Header().Set("Retry-After", "30")
		httpx.WriteError(w, r, httpx.NewCustomError(http.StatusTooManyRequests, "RATE_LIMITED",
			"Terlalu banyak koneksi notifikasi yang terbuka. Tutup tab atau perangkat lain, lalu coba lagi.", nil, httpx.ErrRateLimited))
		return
	}
	defer sub.Close()

	select {
	case <-h.hub.Ready():
	case <-r.Context().Done():
		return
	case <-time.After(10 * time.Second):
		httpx.WriteError(w, r, fmt.Errorf("notification listener not ready"))
		return
	}

	unread, err := h.svc.UnreadCount(r.Context(), actor.UserID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	hdr := w.Header()
	hdr.Set("Content-Type", "text/event-stream")
	hdr.Set("Cache-Control", "no-cache")
	hdr.Set("X-Accel-Buffering", "no") // tells nginx-style proxies not to buffer
	w.WriteHeader(http.StatusOK)

	send := func(event, id string, payload any) bool {
		data, err := json.Marshal(payload)
		if err != nil {
			h.log.ErrorContext(r.Context(), "encoding stream event", slog.String("error", err.Error()))
			return false
		}
		if id != "" {
			_, err = fmt.Fprintf(w, "id: %s\n", id)
		}
		if err == nil {
			_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		}
		if err == nil {
			err = rc.Flush()
		}
		return err == nil
	}

	if !send("ready", "", map[string]int64{"unread": unread}) {
		return
	}

	heartbeat := time.NewTicker(h.opts.Heartbeat)
	defer heartbeat.Stop()
	lifetime := time.NewTimer(h.opts.MaxLifetime)
	defer lifetime.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-h.hub.Done():
			send("reconnect", "", struct{}{})
			return
		case <-lifetime.C:
			send("reconnect", "", struct{}{})
			return
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil || rc.Flush() != nil {
				return
			}
		case id, ok := <-sub.C:
			if !ok { // dropped for falling behind
				send("resync", "", struct{}{})
				return
			}
			if id == Resync {
				if !send("resync", "", struct{}{}) {
					return
				}
				continue
			}
			n, err := h.svc.Get(r.Context(), actor.UserID, id)
			if err != nil {
				h.log.WarnContext(r.Context(), "loading notification for stream", slog.String("error", err.Error()))
				continue
			}
			if !send("notification", n.ID.String(), n) {
				return
			}
		}
	}
}
