// Check-in and violation rules as the browser sees them (brief BR-11, BR-14). The server reads the
// same numbers from its env (CHECKIN_EARLY_MIN, CHECKIN_LATE_MIN, BLOCK_THRESHOLD_POINTS,
// BLOCK_DAYS) and stays the authority: these only decide what the UI offers and says.
export const attendanceRules = {
  /** Check-in opens this many minutes before `start_at`... */
  checkInEarlyMinutes: 15,
  /** ...and closes this many minutes after it. */
  checkInLateMinutes: 15,
  /** Active violation points that block new bookings. */
  blockThresholdPoints: 6,
  /** How long that block lasts. */
  blockDays: 14,
} as const

// Edit and cancel rules of the owner (brief BR-14, BR-26). The server reads EDIT_CUTOFF_MIN and
// LATE_CANCEL_HOURS from its env and stays the authority.
export const bookingChangeRules = {
  /** The owner may edit until this many minutes before the start. */
  editCutoffMinutes: 60,
  /** Cancelling an approved booking closer than this to the start costs a violation point. */
  lateCancelHours: 2,
} as const
