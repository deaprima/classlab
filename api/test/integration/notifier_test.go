package app_test

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"classlab/api/internal/notifications"
	"classlab/api/internal/platform/db/dbgen"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// The types a client can receive are listed in the OpenAPI document; a type added in code but not there
// would make the contract test reject the first response that carries it.
func TestNotificationTypesMatchTheContract(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join(filepath.Dir(file), "..", "..", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	props := doc.Components.Schemas["Notification"].Value.Properties

	enumOf := func(field string) []string {
		t.Helper()
		var out []string
		for _, v := range props[field].Value.Enum {
			out = append(out, v.(string))
		}
		return out
	}
	got, want := enumOf("type"), notifications.Types()
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("Notification.type enum = %v; want the types in code %v", got, want)
	}
	if refs := enumOf("ref_type"); !slices.Equal(refs, []string{notifications.RefBooking, notifications.RefReport}) {
		t.Errorf("Notification.ref_type enum = %v", refs)
	}
}

func TestInAppDeliversToOneUser(t *testing.T) {
	e := newEnv(t)
	n := notifications.NewNotifier(notifications.InApp{})
	q := dbgen.New(e.pool)
	student := uuid.MustParse(e.userIDOf(mahasiswaA))
	ref := uuid.New()

	if err := n.ToUser(t.Context(), q, student, notifications.Message{
		Type: notifications.TypeBookingApproved, Title: "t", Body: "b", RefType: notifications.RefBooking, RefID: ref,
	}, e.clock.Now()); err != nil {
		t.Fatal(err)
	}
	if err := n.ToUser(t.Context(), q, student, notifications.Message{Type: notifications.TypeBookingBlocked, Title: "t", Body: "b"}, e.clock.Now()); err != nil {
		t.Fatal(err)
	}

	var refType, refID *string
	row := func(typ string) {
		t.Helper()
		if err := e.pool.QueryRow(t.Context(), "SELECT ref_type, ref_id::text FROM notifications WHERE user_id = $1 AND type = $2", student, typ).Scan(&refType, &refID); err != nil {
			t.Fatalf("reading %s: %v", typ, err)
		}
	}
	row(notifications.TypeBookingApproved)
	if refType == nil || *refType != "BOOKING" || refID == nil || *refID != ref.String() {
		t.Errorf("with a reference: %v %v; want BOOKING %s", refType, refID, ref)
	}
	row(notifications.TypeBookingBlocked)
	if refType != nil || refID != nil {
		t.Errorf("without a reference: %v %v; want NULL columns", refType, refID)
	}
	if got := e.countRows("SELECT count(*) FROM notifications WHERE user_id <> $1", student); got != 0 {
		t.Errorf("%d notifications reached other users", got)
	}
}

// Who is told follows the policy of the room type (BR-10), read at the moment of sending; people who are
// inactive, and the person excluded, get nothing.
func TestInAppApproversFollowTheRoomTypePolicy(t *testing.T) {
	e := newEnv(t)
	n := notifications.NewNotifier(notifications.InApp{})
	q := dbgen.New(e.pool)

	tu, kalab, admin := e.userIDOf(tuID), e.userIDOf(kalabID), e.userIDOf(adminID)
	secondTU := e.addTU("198505052010012004")
	inactiveTU := e.addTU("198505052010012005")
	if _, err := e.pool.Exec(t.Context(), "UPDATE users SET is_active = false WHERE id = $1", inactiveTU); err != nil {
		t.Fatal(err)
	}

	msg := func(ref uuid.UUID) notifications.Message {
		return notifications.Message{Type: notifications.TypeBookingCreated, Title: "t", Body: "b", RefType: notifications.RefBooking, RefID: ref}
	}
	recipients := func(ref uuid.UUID) []string {
		t.Helper()
		rows, err := e.pool.Query(t.Context(), "SELECT user_id::text FROM notifications WHERE ref_id = $1", ref)
		if err != nil {
			t.Fatal(err)
		}
		ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			t.Fatal(err)
		}
		slices.Sort(ids)
		return ids
	}
	sorted := func(ids ...string) []string { slices.Sort(ids); return ids }

	for _, tc := range []struct {
		typ     string
		exclude string
		want    []string
	}{
		{"KELAS", "", sorted(tu, secondTU)},
		{"KELAS", tu, sorted(secondTU)},
		{"LAB", "", sorted(kalab)},
		{"LAB", kalab, nil},
		{"AUDITORIUM", "", sorted(tu, secondTU, admin)},
		{"RUANG_UJIAN", admin, sorted(tu, secondTU)},
	} {
		ref := uuid.New()
		exclude := uuid.Nil
		if tc.exclude != "" {
			exclude = uuid.MustParse(tc.exclude)
		}
		if err := n.ToApprovers(t.Context(), q, tc.typ, msg(ref), exclude, e.clock.Now()); err != nil {
			t.Fatalf("%s: %v", tc.typ, err)
		}
		if got := recipients(ref); !slices.Equal(got, tc.want) {
			t.Errorf("%s excluding %q: recipients %v; want %v", tc.typ, tc.exclude, got, tc.want)
		}
	}
}

// A notification written inside a transaction that rolls back never exists (agent.md section 7, #7).
func TestInAppRollsBackWithItsTransaction(t *testing.T) {
	e := newEnv(t)
	n := notifications.NewNotifier(notifications.InApp{})
	student := uuid.MustParse(e.userIDOf(mahasiswaA))
	errRollBack := errors.New("roll back")

	err := e.pool.WithTx(context.Background(), func(tx pgx.Tx) error {
		q := dbgen.New(e.pool).WithTx(tx)
		if err := n.ToUser(t.Context(), q, student, notifications.Message{Type: notifications.TypeBookingApproved, Title: "t", Body: "b"}, e.clock.Now()); err != nil {
			return err
		}
		if err := n.ToApprovers(t.Context(), q, "KELAS", notifications.Message{Type: notifications.TypeBookingCreated, Title: "t", Body: "b"}, uuid.Nil, e.clock.Now()); err != nil {
			return err
		}
		return errRollBack
	})
	if !errors.Is(err, errRollBack) {
		t.Fatalf("transaction error = %v; want the rollback error", err)
	}
	if got := e.countRows("SELECT count(*) FROM notifications"); got != 0 {
		t.Errorf("%d notifications survived the rollback", got)
	}
}
