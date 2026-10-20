package notifications

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"classlab/api/internal/platform/db/dbgen"

	"github.com/google/uuid"
)

// recorder is a Medium that remembers what it was asked to deliver. It never touches the queries, so the
// tests here need no database.
type recorder struct {
	name  string
	log   *[]string
	fails error
}

func (r recorder) ToUser(_ context.Context, _ *dbgen.Queries, _ uuid.UUID, m Message, _ time.Time) error {
	*r.log = append(*r.log, r.name+":user:"+m.Type)
	return r.fails
}

func (r recorder) ToApprovers(_ context.Context, _ *dbgen.Queries, roomType string, m Message, _ uuid.UUID, _ time.Time) error {
	*r.log = append(*r.log, r.name+":approvers:"+roomType+":"+m.Type)
	return r.fails
}

func TestNotifierHandsEveryMessageToEveryMediumInOrder(t *testing.T) {
	var log []string
	n := NewNotifier(recorder{name: "a", log: &log}, recorder{name: "b", log: &log})
	ctx := context.Background()

	if err := n.ToUser(ctx, nil, uuid.New(), Message{Type: TypeBookingApproved}, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if err := n.ToApprovers(ctx, nil, "LAB", Message{Type: TypeReportCreated}, uuid.Nil, time.Time{}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"a:user:BOOKING_APPROVED", "b:user:BOOKING_APPROVED",
		"a:approvers:LAB:REPORT_CREATED", "b:approvers:LAB:REPORT_CREATED",
	}
	if !slices.Equal(log, want) {
		t.Errorf("deliveries = %v; want %v", log, want)
	}
}

// A medium that fails must fail the call, because the caller's transaction then rolls back; later media
// are not asked, so nothing is half-delivered.
func TestNotifierStopsAtTheFirstFailingMedium(t *testing.T) {
	boom := errors.New("boom")
	var log []string
	n := NewNotifier(recorder{name: "a", log: &log}, recorder{name: "b", log: &log, fails: boom}, recorder{name: "c", log: &log})
	ctx := context.Background()

	if err := n.ToUser(ctx, nil, uuid.New(), Message{Type: TypeBookingApproved}, time.Time{}); !errors.Is(err, boom) {
		t.Errorf("ToUser error = %v; want it to wrap %v", err, boom)
	}
	if err := n.ToApprovers(ctx, nil, "KELAS", Message{Type: TypeBookingCreated}, uuid.Nil, time.Time{}); !errors.Is(err, boom) {
		t.Errorf("ToApprovers error = %v; want it to wrap %v", err, boom)
	}
	for _, d := range log {
		if d[0] == 'c' {
			t.Errorf("medium c was called after b failed: %v", log)
		}
	}
}

func TestNotifierWithoutMediaDoesNothing(t *testing.T) {
	n := NewNotifier()
	if err := n.ToUser(context.Background(), nil, uuid.New(), Message{}, time.Time{}); err != nil {
		t.Errorf("ToUser = %v", err)
	}
	if err := n.ToApprovers(context.Background(), nil, "KELAS", Message{}, uuid.Nil, time.Time{}); err != nil {
		t.Errorf("ToApprovers = %v", err)
	}
}

func TestTypesAreUniqueAndComplete(t *testing.T) {
	got := Types()
	seen := map[string]bool{}
	for _, typ := range got {
		if typ == "" || seen[typ] {
			t.Errorf("type %q is empty or listed twice", typ)
		}
		seen[typ] = true
	}
	// The list is the brief's table (section 5): every constant must be in it.
	for _, typ := range []string{
		TypeBookingCreated, TypeBookingApproved, TypeBookingRejected, TypeBookingCancelled, TypeBookingEdited,
		TypeBookingNoShow, TypeBookingAutoClosed, TypeBookingBlocked, TypeBookingExpired,
		TypeReminderCheckin, TypeReminderCheckinLast, TypeReminderCheckout, TypeReportCreated, TypeReportStatusChanged,
	} {
		if !seen[typ] {
			t.Errorf("Types() is missing %s", typ)
		}
	}
	if len(got) != 14 {
		t.Errorf("Types() has %d entries; want 14", len(got))
	}
}
