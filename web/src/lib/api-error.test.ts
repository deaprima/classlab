import { AxiosError, AxiosHeaders } from 'axios'
import { apiErrorBody } from '@/test-utils/mock-adapter'
import { describe, expect, it, vi } from 'vitest'
import {
  applyFieldErrors,
  getApiError,
  getErrorMessage,
  isApiErrorCode,
} from './api-error'

function axiosError(status: number, data: unknown) {
  const config = { headers: new AxiosHeaders() }
  return new AxiosError(
    'failed',
    'ERR_BAD_REQUEST',
    config,
    {},
    {
      status,
      statusText: '',
      headers: {},
      config,
      data,
    }
  )
}

describe('getApiError', () => {
  it('reads the standard error body', () => {
    const err = axiosError(
      409,
      apiErrorBody('BOOKING_CONFLICT', 'Ruang sudah terpakai.', [
        { kind: 'BOOKING', status: 'DISETUJUI' },
      ])
    )

    expect(getApiError(err)).toEqual({
      status: 409,
      code: 'BOOKING_CONFLICT',
      message: 'Ruang sudah terpakai.',
      details: [{ kind: 'BOOKING', status: 'DISETUJUI' }],
      requestId: 'req_test',
    })
    expect(isApiErrorCode(err, 'BOOKING_CONFLICT')).toBe(true)
  })

  it('returns null for bodies that are not API errors', () => {
    expect(getApiError(axiosError(502, '<html>Bad Gateway</html>'))).toBeNull()
    expect(getApiError(new Error('boom'))).toBeNull()
  })
})

describe('getErrorMessage', () => {
  it('prefers the server message', () => {
    const err = axiosError(
      401,
      apiErrorBody('UNAUTHENTICATED', 'NIM/NIP atau kata sandi salah.')
    )
    expect(getErrorMessage(err)).toBe('NIM/NIP atau kata sandi salah.')
  })

  it('explains a network failure', () => {
    const err = new AxiosError('Network Error', 'ERR_NETWORK')
    expect(getErrorMessage(err)).toMatch(/Tidak dapat terhubung ke server/)
  })

  it('explains a server failure without an API body', () => {
    expect(getErrorMessage(axiosError(502, ''))).toMatch(/Server sedang/)
  })
})

describe('applyFieldErrors', () => {
  it('maps API fields to form fields and returns the rest', () => {
    const setError = vi.fn()
    const err = axiosError(
      422,
      apiErrorBody('VALIDATION_ERROR', 'Data tidak valid.', [
        { field: 'start_at', message: 'Harus kelipatan 30 menit.' },
        { field: 'purpose', message: 'Wajib diisi.' },
        { message: 'Kuota pengajuan aktif habis.' },
      ])
    )

    const rest = applyFieldErrors(err, setError, {
      start_at: 'startTime',
      purpose: 'purpose',
    })

    expect(setError).toHaveBeenCalledWith(
      'startTime',
      { message: 'Harus kelipatan 30 menit.' },
      { shouldFocus: true }
    )
    expect(setError).toHaveBeenCalledWith(
      'purpose',
      { message: 'Wajib diisi.' },
      { shouldFocus: false }
    )
    expect(rest).toEqual([{ message: 'Kuota pengajuan aktif habis.' }])
  })
})
