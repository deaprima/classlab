package notifications

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
)

// testHub returns a hub that never connects to a database: only the fan-out logic is under test.
func testHub() *Hub {
	h := NewHub(context.Background(), "", slog.New(slog.NewTextHandler(io.Discard, nil)))
	h.startOnce.Do(func() {}) // Subscribe must not start the real listener
	return h
}

func TestParsePayload(t *testing.T) {
	user, id := uuid.New(), uuid.New()
	gotUser, gotID, ok := parsePayload(user.String() + ":" + id.String())
	if !ok || gotUser != user || gotID != id {
		t.Errorf("parsePayload = %v %v %v", gotUser, gotID, ok)
	}
	for _, bad := range []string{"", "no-colon", user.String() + ":", ":" + id.String(), "x:y", user.String() + ":" + id.String() + ":extra"} {
		if _, _, ok := parsePayload(bad); ok {
			t.Errorf("parsePayload(%q) accepted", bad)
		}
	}
}

func TestDeliverReachesOnlyTheNamedUser(t *testing.T) {
	h := testHub()
	alice, bob := uuid.New(), uuid.New()
	a1, a2, b1 := h.Subscribe(alice), h.Subscribe(alice), h.Subscribe(bob)
	defer a1.Close()
	defer a2.Close()
	defer b1.Close()

	id := uuid.New()
	h.deliver(alice, id)

	for name, s := range map[string]*Subscription{"first alice stream": a1, "second alice stream": a2} {
		select {
		case got := <-s.C:
			if got != id {
				t.Errorf("%s got %v; want %v", name, got, id)
			}
		default:
			t.Errorf("%s received nothing", name)
		}
	}
	select {
	case got := <-b1.C:
		t.Errorf("bob received alice's notification %v", got)
	default:
	}
}

// A stream that stops reading must be dropped, not allowed to block delivery to everyone else.
func TestSlowSubscriberIsDroppedWithoutBlockingOthers(t *testing.T) {
	h := testHub()
	user := uuid.New()
	slow, fast := h.Subscribe(user), h.Subscribe(user)
	defer fast.Close()

	for range subscriberBuffer {
		h.deliver(user, uuid.New())
		<-fast.C // the fast reader keeps up
	}
	h.deliver(user, uuid.New()) // one more than the slow reader's buffer holds

	if _, open := <-fast.C; !open {
		t.Error("the fast subscriber was dropped")
	}
	drained := 0
	for range slow.C { // closed after its buffered items
		drained++
	}
	if drained != subscriberBuffer {
		t.Errorf("slow subscriber drained %d items; want its %d buffered ones and then a closed channel", drained, subscriberBuffer)
	}
	slow.Close() // closing a dropped subscription must be harmless
}

func TestBroadcastReachesEveryone(t *testing.T) {
	h := testHub()
	subs := []*Subscription{h.Subscribe(uuid.New()), h.Subscribe(uuid.New())}
	h.broadcast(Resync)
	for i, s := range subs {
		select {
		case got := <-s.C:
			if got != Resync {
				t.Errorf("subscriber %d got %v; want Resync", i, got)
			}
		default:
			t.Errorf("subscriber %d missed the broadcast", i)
		}
		s.Close()
	}
	if len(h.subs) != 0 {
		t.Errorf("%d users still registered after every subscription closed", len(h.subs))
	}
}
