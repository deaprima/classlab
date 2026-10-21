import { getApiError, getErrorMessage } from '@/lib/api-error'
import { type CheckoutCondition } from '@/lib/api-types'

export const CHECKOUT_NOTE_MAX = 2000

export const CONDITION_LABELS: Record<CheckoutCondition, string> = {
  NORMAL: 'Bersih dan kondisi baik',
  ADA_MASALAH: 'Ada kerusakan',
}

/** Why a check-in did not go through, in the four cases the Figma frames show (DESIGN.md). */
export type CheckInProblem =
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

export type CheckoutDraft = {
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
