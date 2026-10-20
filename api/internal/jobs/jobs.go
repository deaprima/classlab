// Package jobs runs the scheduled work of the booking lifecycle (brief BR-13): bookings nobody showed up for,
// bookings nobody checked out of, requests that waited too long, reminders, and the removal of old files.
//
// Every step is idempotent and safe with several instances running: it selects the booking rows it acts on
// with FOR UPDATE SKIP LOCKED inside the transaction that changes them, so a row is never handled twice and
// a row someone else is working on (a cancellation, a check-in, another instance) is left for the next tick.
// RunOnce takes the time as an argument so tests drive it with an injected clock.
package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"classlab/api/internal/attachments"
	"classlab/api/internal/notifications"
	"classlab/api/internal/platform/clock"
	"classlab/api/internal/platform/db"
	"classlab/api/internal/platform/db/dbgen"
	"classlab/api/internal/violations"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	// batchSize bounds how many rows one transaction locks, so a backlog is worked off in short transactions.
	batchSize = 200

	// purgeEvery is how often the file purge runs (BR-13 c: daily).
	purgeEvery = 24 * time.Hour
)

// Settings are the BR-11 to BR-28 values the jobs read from configuration.
type Settings struct {
	CheckinEarly        time.Duration // BR-11
	CheckinLate         time.Duration // BR-11, BR-13 a
	CheckoutGrace       time.Duration // BR-13 b
	ReminderCheckinLast time.Duration // BR-28
	ReminderCheckout    time.Duration // BR-28
	PointsNoShow        int           // BR-14
	PointsNoCheckout    int           // BR-14
	Violations          violations.Settings
	Purge               attachments.PurgePolicy
}

// Result counts what one run did.
type Result struct {
	NoShows         int
	MissedCheckouts int
	Expired         int
	Reminders       int
	PurgedFiles     int
}

type Runner struct {
	pool     *db.Pool
	q        *dbgen.Queries
	clock    clock.Clock
	notifier *notifications.Notifier
	files    *attachments.Service
	settings Settings
	log      *slog.Logger

	mu        sync.Mutex
	lastPurge time.Time
}

func New(pool *db.Pool, clk clock.Clock, notifier *notifications.Notifier, files *attachments.Service, settings Settings, log *slog.Logger) *Runner {
	return &Runner{pool: pool, q: dbgen.New(pool), clock: clk, notifier: notifier, files: files, settings: settings, log: log}
}

// RunOnce does one pass of every step at the given time. The status changes come first, so a booking that
// just stopped being approved or in use is not reminded about. The first failing step ends the run with its
// error; the steps already done stay done, and the failed one is retried by the next tick.
func (r *Runner) RunOnce(ctx context.Context, now time.Time) (Result, error) {
	var res Result
	var err error

	if res.NoShows, err = r.markNoShows(ctx, now); err != nil {
		return res, err
	}
	if res.MissedCheckouts, err = r.markMissedCheckouts(ctx, now); err != nil {
		return res, err
	}
	if res.Expired, err = r.expireRequests(ctx, now); err != nil {
		return res, err
	}
	if res.Reminders, err = r.sendReminders(ctx, now); err != nil {
		return res, err
	}
	if res.PurgedFiles, err = r.purgeIfDue(ctx, now); err != nil {
		return res, err
	}
	return res, nil
}

// Run calls RunOnce every interval until ctx ends. A failed run is logged and retried at the next tick:
// housekeeping must never take the server down.
func (r *Runner) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		res, err := r.RunOnce(ctx, r.clock.Now())
		switch {
		case ctx.Err() != nil:
			return
		case err != nil:
			r.log.ErrorContext(ctx, "scheduled jobs failed", slog.String("error", err.Error()))
		case res != Result{}:
			r.log.InfoContext(ctx, "scheduled jobs ran",
				slog.Int("no_shows", res.NoShows), slog.Int("missed_checkouts", res.MissedCheckouts),
				slog.Int("expired", res.Expired), slog.Int("reminders", res.Reminders), slog.Int("purged_files", res.PurgedFiles))
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// due is a locked booking a step is about to change.
type due = dbgen.LockNoShowsRow

// batches calls fetch inside a fresh transaction until it returns nothing, passing each locked batch to
// handle in the same transaction. Rows that handle changes leave the selection, which is what ends the loop.
func (r *Runner) batches(ctx context.Context, fetch func(q *dbgen.Queries) ([]due, error), handle func(q *dbgen.Queries, rows []due) error) (int, error) {
	total := 0
	for {
		n := 0
		err := r.pool.WithTx(ctx, func(tx pgx.Tx) error {
			q := r.q.WithTx(tx)
			rows, err := fetch(q)
			if err != nil {
				return err
			}
			rows = wholeGroups(rows)
			n = len(rows)
			if n == 0 {
				return nil
			}
			return handle(q, rows)
		})
		if err != nil {
			return total, err
		}
		total += n
		if n == 0 {
			return total, nil
		}
	}
}

// wholeGroups drops the rows at the end of a full batch that belong to a multi-date request cut in the
// middle, so the request is handled whole in the next batch and its owner gets one notification (BR-27).
func wholeGroups(rows []due) []due {
	if len(rows) < batchSize || !rows[len(rows)-1].GroupID.Valid {
		return rows
	}
	last := rows[len(rows)-1].GroupID
	keep := len(rows)
	for keep > 0 && rows[keep-1].GroupID == last {
		keep--
	}
	return rows[:keep]
}

// systemEvent records a change of status made by the system: no actor (brief 4.1).
func systemEvent(ctx context.Context, q *dbgen.Queries, id uuid.UUID, from, to dbgen.BookingStatus, reason string, now time.Time) error {
	if err := q.SystemSetBookingStatus(ctx, dbgen.SystemSetBookingStatusParams{ID: id, Status: to, Now: now}); err != nil {
		return fmt.Errorf("updating booking status: %w", err)
	}
	if err := q.CreateBookingEvent(ctx, dbgen.CreateBookingEventParams{
		BookingID: id, FromStatus: dbgen.NullBookingStatus{BookingStatus: from, Valid: true}, ToStatus: to,
		Reason: pgtype.Text{String: reason, Valid: reason != ""}, Kind: "STATUS", CreatedAt: now,
	}); err != nil {
		return fmt.Errorf("recording booking event: %w", err)
	}
	return nil
}

// slotText renders a booking's slot for people, e.g. "20/10/2026 13:00-15:00 WIB".
func slotText(start, end time.Time) string {
	return fmt.Sprintf("%s-%s WIB", clock.ToWIB(start).Format("02/01/2006 15:04"), clock.ToWIB(end).Format("15:04"))
}

// markNoShows: DISETUJUI past the check-in window becomes TIDAK_HADIR with violation points (BR-13 a).
func (r *Runner) markNoShows(ctx context.Context, now time.Time) (int, error) {
	return r.batches(ctx,
		func(q *dbgen.Queries) ([]due, error) {
			rows, err := q.LockNoShows(ctx, dbgen.LockNoShowsParams{Cutoff: now.Add(-r.settings.CheckinLate), Batch: batchSize})
			if err != nil {
				return nil, fmt.Errorf("selecting no-shows: %w", err)
			}
			return rows, nil
		},
		func(q *dbgen.Queries, rows []due) error {
			for _, b := range rows {
				if err := systemEvent(ctx, q, b.ID, dbgen.BookingStatusDISETUJUI, dbgen.BookingStatusTIDAKHADIR, "NO_SHOW", now); err != nil {
					return err
				}
				if err := violations.Record(ctx, q, r.notifier, r.settings.Violations, b.RequesterID, b.ID,
					dbgen.ViolationKindTIDAKHADIR, r.settings.PointsNoShow, now); err != nil {
					return err
				}
				if err := r.notifier.ToUser(ctx, q, b.RequesterID, notifications.Message{
					Type: notifications.TypeBookingNoShow, Title: "Booking tidak dipakai",
					Body: fmt.Sprintf("Anda tidak check-in di %s pada %s, sehingga booking dilepas dan Anda mendapat %d poin pelanggaran.",
						b.RoomCode, slotText(b.StartAt, b.EndAt), r.settings.PointsNoShow),
					RefType: notifications.RefBooking, RefID: b.ID,
				}, now); err != nil {
					return err
				}
			}
			return nil
		})
}

// markMissedCheckouts: BERLANGSUNG past end_at + grace becomes TANPA_CHECKOUT with a point (BR-13 b).
func (r *Runner) markMissedCheckouts(ctx context.Context, now time.Time) (int, error) {
	return r.batches(ctx,
		func(q *dbgen.Queries) ([]due, error) {
			rows, err := q.LockMissedCheckouts(ctx, dbgen.LockMissedCheckoutsParams{Cutoff: now.Add(-r.settings.CheckoutGrace), Batch: batchSize})
			if err != nil {
				return nil, fmt.Errorf("selecting missed check-outs: %w", err)
			}
			out := make([]due, len(rows))
			for i, row := range rows {
				out[i] = due(row)
			}
			return out, nil
		},
		func(q *dbgen.Queries, rows []due) error {
			for _, b := range rows {
				if err := systemEvent(ctx, q, b.ID, dbgen.BookingStatusBERLANGSUNG, dbgen.BookingStatusTANPACHECKOUT, "NO_CHECKOUT", now); err != nil {
					return err
				}
				if err := violations.Record(ctx, q, r.notifier, r.settings.Violations, b.RequesterID, b.ID,
					dbgen.ViolationKindTANPACHECKOUT, r.settings.PointsNoCheckout, now); err != nil {
					return err
				}
				if err := r.notifier.ToUser(ctx, q, b.RequesterID, notifications.Message{
					Type: notifications.TypeBookingAutoClosed, Title: "Booking ditutup otomatis",
					Body: fmt.Sprintf("Anda tidak check-out dari %s (%s), sehingga booking ditutup otomatis dan Anda mendapat %d poin pelanggaran.",
						b.RoomCode, slotText(b.StartAt, b.EndAt), r.settings.PointsNoCheckout),
					RefType: notifications.RefBooking, RefID: b.ID,
				}, now); err != nil {
					return err
				}
			}
			return nil
		})
}

// expireRequests: MENUNGGU past expires_at becomes KEDALUWARSA, without points (BR-22). A multi-date request
// expires together and its owner is told once.
func (r *Runner) expireRequests(ctx context.Context, now time.Time) (int, error) {
	return r.batches(ctx,
		func(q *dbgen.Queries) ([]due, error) {
			rows, err := q.LockExpiredRequests(ctx, dbgen.LockExpiredRequestsParams{Now: pgtype.Timestamptz{Time: now, Valid: true}, Batch: batchSize})
			if err != nil {
				return nil, fmt.Errorf("selecting expired requests: %w", err)
			}
			out := make([]due, len(rows))
			for i, row := range rows {
				out[i] = due(row)
			}
			return out, nil
		},
		func(q *dbgen.Queries, rows []due) error {
			for _, b := range rows {
				if err := systemEvent(ctx, q, b.ID, dbgen.BookingStatusMENUNGGU, dbgen.BookingStatusKEDALUWARSA, "EXPIRED", now); err != nil {
					return err
				}
			}
			// One notification per request: the first row stands for its group.
			notified := map[uuid.UUID]bool{}
			for i, b := range rows {
				if b.GroupID.Valid {
					g := uuid.UUID(b.GroupID.Bytes)
					if notified[g] {
						continue
					}
					notified[g] = true
				}
				dates := 1
				if b.GroupID.Valid {
					for _, o := range rows[i:] {
						if o.GroupID == b.GroupID && o.ID != b.ID {
							dates++
						}
					}
				}
				when := "pada " + slotText(b.StartAt, b.EndAt)
				if dates > 1 {
					when = fmt.Sprintf("untuk %d tanggal mulai %s", dates, slotText(b.StartAt, b.EndAt))
				}
				if err := r.notifier.ToUser(ctx, q, b.RequesterID, notifications.Message{
					Type: notifications.TypeBookingExpired, Title: "Pengajuan kedaluwarsa",
					Body:    fmt.Sprintf("Pengajuan %s %s tidak diputuskan tepat waktu dan kedaluwarsa. Anda dapat mengajukan ulang.", b.RoomCode, when),
					RefType: notifications.RefBooking, RefID: b.ID,
				}, now); err != nil {
					return err
				}
			}
			return nil
		})
}

// sendReminders creates the in-app reminders of BR-28. Each is unique per (user, type, booking): the select
// leaves out bookings already reminded, and the unique index is the safeguard behind it.
func (r *Runner) sendReminders(ctx context.Context, now time.Time) (int, error) {
	s := r.settings
	total := 0

	// REMINDER_CHECKIN: the check-in window is open and the booking has not been checked in.
	n, err := r.remindCheckin(ctx, now, notifications.TypeReminderCheckin, now.Add(-s.CheckinLate), now.Add(s.CheckinEarly),
		"Check-in sudah dibuka", func(code string, start time.Time) string {
			return fmt.Sprintf("Anda dapat check-in di %s untuk booking pukul %s WIB dengan memindai QR di pintu.", code, clock.ToWIB(start).Format("15:04"))
		})
	total += n
	if err != nil {
		return total, err
	}

	// REMINDER_CHECKIN_LAST: the window closes in ReminderCheckinLast and the booking is still not checked in.
	n, err = r.remindCheckin(ctx, now, notifications.TypeReminderCheckinLast, now.Add(-s.CheckinLate), now.Add(-s.CheckinLate+s.ReminderCheckinLast),
		"Check-in hampir ditutup", func(code string, start time.Time) string {
			return fmt.Sprintf("Check-in untuk %s ditutup pukul %s WIB. Setelah itu booking dilepas dan dihitung sebagai tidak hadir.",
				code, clock.ToWIB(start.Add(s.CheckinLate)).Format("15:04"))
		})
	total += n
	if err != nil {
		return total, err
	}

	// REMINDER_CHECKOUT: the booking is in use and ends within ReminderCheckout.
	for {
		count := 0
		err := r.pool.WithTx(ctx, func(tx pgx.Tx) error {
			q := r.q.WithTx(tx)
			rows, err := q.LockCheckoutReminderCandidates(ctx, dbgen.LockCheckoutReminderCandidatesParams{
				EndFrom: now.Add(-s.CheckoutGrace), EndTo: now.Add(s.ReminderCheckout),
				ReminderType: notifications.TypeReminderCheckout, Batch: batchSize,
			})
			if err != nil {
				return fmt.Errorf("selecting check-out reminders: %w", err)
			}
			for _, b := range rows {
				if err := r.notifier.ToUser(ctx, q, b.RequesterID, notifications.Message{
					Type: notifications.TypeReminderCheckout, Title: "Saatnya check-out",
					Body: fmt.Sprintf("Booking %s berakhir pukul %s WIB. Foto kondisi ruangan lalu check-out agar tidak dihitung sebagai pelanggaran.",
						b.RoomCode, clock.ToWIB(b.EndAt).Format("15:04")),
					RefType: notifications.RefBooking, RefID: b.ID,
				}, now); err != nil {
					return err
				}
			}
			count = len(rows)
			return nil
		})
		if err != nil {
			return total, err
		}
		total += count
		if count == 0 {
			return total, nil
		}
	}
}

// remindCheckin sends one check-in reminder type to approved bookings whose start_at is in [from, to].
func (r *Runner) remindCheckin(ctx context.Context, now time.Time, typ string, from, to time.Time, title string, body func(code string, start time.Time) string) (int, error) {
	total := 0
	for {
		count := 0
		err := r.pool.WithTx(ctx, func(tx pgx.Tx) error {
			q := r.q.WithTx(tx)
			rows, err := q.LockCheckinReminderCandidates(ctx, dbgen.LockCheckinReminderCandidatesParams{
				StartFrom: from, StartTo: to, ReminderType: typ, Batch: batchSize,
			})
			if err != nil {
				return fmt.Errorf("selecting %s candidates: %w", typ, err)
			}
			for _, b := range rows {
				if err := r.notifier.ToUser(ctx, q, b.RequesterID, notifications.Message{
					Type: typ, Title: title, Body: body(b.RoomCode, b.StartAt),
					RefType: notifications.RefBooking, RefID: b.ID,
				}, now); err != nil {
					return err
				}
			}
			count = len(rows)
			return nil
		})
		if err != nil {
			return total, err
		}
		total += count
		if count == 0 {
			return total, nil
		}
	}
}

// purgeIfDue removes old files once a day (BR-13 c). The time of the last purge is kept in memory: after a
// restart the purge simply runs again, which costs one cheap query.
func (r *Runner) purgeIfDue(ctx context.Context, now time.Time) (int, error) {
	r.mu.Lock()
	needed := r.lastPurge.IsZero() || now.Sub(r.lastPurge) >= purgeEvery
	r.mu.Unlock()
	if !needed {
		return 0, nil
	}
	n, err := r.PurgeFiles(ctx, now)
	if err != nil {
		return n, err
	}
	r.mu.Lock()
	r.lastPurge = now
	r.mu.Unlock()
	return n, nil
}

// PurgeFiles deletes the files past their retention and the orphans, regardless of when it last ran.
func (r *Runner) PurgeFiles(ctx context.Context, now time.Time) (int, error) {
	n, err := r.files.PurgeExpired(ctx, now, r.settings.Purge, batchSize)
	if err != nil {
		return n, fmt.Errorf("purging attachments: %w", err)
	}
	return n, nil
}
