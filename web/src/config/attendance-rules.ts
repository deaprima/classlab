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
