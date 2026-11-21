import { getApiError, getErrorMessage } from '@/lib/api-error'
import { type CheckoutCondition } from '@/lib/api-types'

export const CHECKOUT_NOTE_MAX = 2000

const CHECK_IN_PATH = '/check-in'
const QR_FIELD_MAX = 128

/**
 * Reads the room code and token out of what a door QR encodes (`<frontend>/check-in?room=&t=`).
 * Only those two values are taken: the in-app scanner never follows the scanned address, so a QR
 * from anywhere else cannot send the person away from the app. The origin is not compared because
 * the QR is printed with the server's configured frontend address, which may differ from the one
 * the person is on (a tunnel, a LAN name).
 */
export function parseCheckInQr(
  text: string
): { room: string; token: string } | null {
  let url: URL
  try {
    url = new URL(text.trim())
  } catch {
    return null
  }
  if (url.protocol !== 'https:' && url.protocol !== 'http:') return null
  if (url.pathname.replace(/\/+$/, '') !== CHECK_IN_PATH) return null
  const room = url.searchParams.get('room')?.trim()
  const token = url.searchParams.get('t')?.trim()
  if (!room || !token) return null
  if (room.length > QR_FIELD_MAX || token.length > QR_FIELD_MAX) return null
  return { room, token }
}

export const CONDITION_LABELS: Record<CheckoutCondition, string> = {
  NORMAL: 'Bersih dan kondisi baik',
  ADA_MASALAH: 'Ada kerusakan',
}

/** Why a check-in did not go through, in the four cases the Figma frames show (DESIGN.md). */
type CheckInProblem =
  | { kind: 'qr-invalid' }
  | { kind: 'outside-window'; opensAt: string; closesAt: string }
  /** Nothing approved for this person in this room right now (someone else's booking, or none). */
  | { kind: 'not-owner' }
  | { kind: 'other'; message: string }

/**
 * `next_window` arrives as two field/message pairs (decisions.md, Phase 5): the person has a later
 * booking in this room, so the problem is the time, not the room.
 */
export function classifyCheckInError(error: unknown): CheckInProblem {
  const apiError = getApiError(error)
  if (apiError?.code === 'QR_INVALID') return { kind: 'qr-invalid' }
  if (apiError?.code === 'NO_ELIGIBLE_BOOKING') {
    const at = (field: string) =>
      apiError.details.find((d) => d.field === field)?.message
    const opensAt = at('next_window.opens_at')
    const closesAt = at('next_window.closes_at')
    if (opensAt && closesAt)
      return { kind: 'outside-window', opensAt, closesAt }
    return { kind: 'not-owner' }
  }
  return { kind: 'other', message: getErrorMessage(error) }
}

type CheckoutDraft = {
  condition: CheckoutCondition | null
  note: string
  photo: Blob | null
}

export type CheckoutErrors = Partial<
  Record<'condition' | 'note' | 'photo', string>
>

/** Mirrors the server's rules (BR-12) so the user hears about a gap before the upload. */
export function validateCheckout(draft: CheckoutDraft): CheckoutErrors {
  const errors: CheckoutErrors = {}
  if (!draft.condition) errors.condition = 'Pilih kondisi ruang.'
  const note = draft.note.trim()
  if (draft.condition === 'ADA_MASALAH' && !note) {
    errors.note = 'Jelaskan kerusakan yang kamu temukan.'
  } else if (note.length > CHECKOUT_NOTE_MAX) {
    errors.note = `Catatan maksimal ${CHECKOUT_NOTE_MAX} karakter.`
  }
  if (!draft.photo) errors.photo = 'Ambil foto kondisi ruang dulu.'
  return errors
}
