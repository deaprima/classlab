package notifications

// Notification types (brief section 5, "Tipe notifikasi"). They are stored as text and shown to clients
// as Notification.type, so a new one is added here, in the OpenAPI enum and in the brief together.
const (
	TypeBookingCreated    = "BOOKING_CREATED"
	TypeBookingApproved   = "BOOKING_APPROVED"
	TypeBookingRejected   = "BOOKING_REJECTED"
	TypeBookingCancelled  = "BOOKING_CANCELLED"
	TypeBookingEdited     = "BOOKING_EDITED"
	TypeBookingNoShow     = "BOOKING_NO_SHOW"
	TypeBookingAutoClosed = "BOOKING_AUTO_CLOSED"
	TypeBookingBlocked    = "BOOKING_BLOCKED"
	TypeBookingExpired    = "BOOKING_EXPIRED"

	TypeReminderCheckin     = "REMINDER_CHECKIN"
	TypeReminderCheckinLast = "REMINDER_CHECKIN_LAST"
	TypeReminderCheckout    = "REMINDER_CHECKOUT"

	TypeReportCreated       = "REPORT_CREATED"
	TypeReportStatusChanged = "REPORT_STATUS_CHANGED"
)

// What a notification points at (Notification.ref_type).
const (
	RefBooking = "BOOKING"
	RefReport  = "REPORT"
)

// Types lists every notification type, in the order of the brief.
func Types() []string {
	return []string{
		TypeBookingCreated, TypeBookingApproved, TypeBookingRejected, TypeBookingCancelled, TypeBookingEdited,
		TypeBookingNoShow, TypeBookingAutoClosed, TypeBookingBlocked, TypeBookingExpired,
		TypeReminderCheckin, TypeReminderCheckinLast, TypeReminderCheckout,
		TypeReportCreated, TypeReportStatusChanged,
	}
}
