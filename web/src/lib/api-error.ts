import { AxiosError } from 'axios'
import {
  type FieldValues,
  type Path,
  type UseFormSetError,
} from 'react-hook-form'
import { type ErrorBody } from './api-types'

type ApiErrorDetail = ErrorBody['error']['details'][number]

type ApiError = {
  status: number
  code: string
  message: string
  details: ApiErrorDetail[]
  requestId?: string
}

/** The server's `{ error: {...} }` body, or null when the failure did not come from the API. */
export function getApiError(error: unknown): ApiError | null {
  if (!(error instanceof AxiosError) || !error.response) return null
  const data: unknown = error.response.data
  if (typeof data !== 'object' || data === null || !('error' in data)) {
    return null
  }
  const body: unknown = data.error
  if (
    typeof body !== 'object' ||
    body === null ||
    !('code' in body) ||
    typeof body.code !== 'string' ||
    !('message' in body) ||
    typeof body.message !== 'string'
  ) {
    return null
  }
  return {
    status: error.response.status,
    code: body.code,
    message: body.message,
    details:
      'details' in body && Array.isArray(body.details) ? body.details : [],
    requestId:
      'request_id' in body && typeof body.request_id === 'string'
        ? body.request_id
        : undefined,
  }
}

export function isApiErrorCode(error: unknown, code: string): boolean {
  return getApiError(error)?.code === code
}

/** A message worth showing to the user; the server's own message wins. */
export function getErrorMessage(error: unknown): string {
  const apiError = getApiError(error)
  if (apiError) return apiError.message

  if (error instanceof AxiosError) {
    if (!error.response) {
      return 'Tidak dapat terhubung ke server. Periksa koneksi Anda lalu coba lagi.'
    }
    if (error.response.status >= 500) {
      return 'Server sedang bermasalah. Coba lagi beberapa saat lagi.'
    }
  }
  return 'Permintaan tidak dapat diproses. Coba lagi.'
}

/**
 * Puts `422` field errors on the matching form fields. `fields` maps the API field name to the form
 * field. Returns the details that matched no field, so the caller can show them elsewhere.
 */
export function applyFieldErrors<T extends FieldValues>(
  error: unknown,
  setError: UseFormSetError<T>,
  fields: Partial<Record<string, Path<T>>>
): ApiErrorDetail[] {
  const apiError = getApiError(error)
  if (!apiError) return []

  const unmatched: ApiErrorDetail[] = []
  let focused = false
  for (const detail of apiError.details) {
    const target = detail.field ? fields[detail.field] : undefined
    if (target && detail.message) {
      setError(target, { message: detail.message }, { shouldFocus: !focused })
      focused = true
    } else {
      unmatched.push(detail)
    }
  }
  return unmatched
}

/**
 * Splits a failed form submission: 422 field details go onto their fields, and whatever is left
 * (a message without a field, a 409 conflict with its clashing items, a network error) is returned
 * for an alert above the form. Null when every problem landed on a field.
 */
export function formProblemFrom<T extends FieldValues>(
  error: unknown,
  setError: UseFormSetError<T>,
  fields: Partial<Record<string, Path<T>>>
): { message: string; conflicts?: ApiErrorDetail[] } | null {
  const apiError = getApiError(error)
  if (apiError?.code === 'BOOKING_CONFLICT') {
    return { message: apiError.message, conflicts: apiError.details }
  }
  const rest = applyFieldErrors(error, setError, fields)
  if (!apiError || apiError.details.length === 0 || rest.length > 0) {
    return {
      message: rest.find((d) => d.message)?.message ?? getErrorMessage(error),
    }
  }
  return null
}
