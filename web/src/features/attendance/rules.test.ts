import { AxiosError, AxiosHeaders } from 'axios'
import { apiErrorBody } from '@/test-utils/mock-adapter'
import { describe, expect, it } from 'vitest'
import { classifyCheckInError, parseCheckInQr, validateCheckout } from './rules'

function failure(
  status: number,
  code: string,
  details: Record<string, string>[] = []
) {
  return new AxiosError(
    'x',
    'ERR_BAD_REQUEST',
    undefined,
    {},
    {
      status,
      statusText: '',
      headers: new AxiosHeaders(),
      config: { headers: new AxiosHeaders() },
      data: apiErrorBody(code, 'pesan server', details),
    }
  )
}

describe('classifyCheckInError', () => {
  it('maps a wrong or unknown QR to qr-invalid', () => {
    expect(classifyCheckInError(failure(422, 'QR_INVALID'))).toEqual({
      kind: 'qr-invalid',
    })
  })

  it('reads the next window out of the details pairs', () => {
    const problem = classifyCheckInError(
      failure(409, 'NO_ELIGIBLE_BOOKING', [
        { field: 'next_window.opens_at', message: '2026-10-20T12:45:00+07:00' },
        {
          field: 'next_window.closes_at',
          message: '2026-10-20T13:15:00+07:00',
        },
      ])
    )
    expect(problem).toEqual({
      kind: 'outside-window',
      opensAt: '2026-10-20T12:45:00+07:00',
      closesAt: '2026-10-20T13:15:00+07:00',
    })
  })

  it('treats no eligible booking without a next window as not-owner', () => {
    expect(classifyCheckInError(failure(409, 'NO_ELIGIBLE_BOOKING'))).toEqual({
      kind: 'not-owner',
    })
  })

  it('falls back to the server message for anything else', () => {
    expect(classifyCheckInError(failure(403, 'FORBIDDEN'))).toEqual({
      kind: 'other',
      message: 'pesan server',
    })
  })
})

describe('validateCheckout', () => {
  const photo = new Blob(['x'], { type: 'image/jpeg' })

  it('needs a condition and a photo', () => {
    expect(
      validateCheckout({ condition: null, note: '', photo: null })
    ).toEqual({
      condition: 'Pilih kondisi ruang.',
      photo: 'Ambil foto kondisi ruang dulu.',
    })
  })

  it('accepts a normal check-out without a note', () => {
    expect(validateCheckout({ condition: 'NORMAL', note: '', photo })).toEqual(
      {}
    )
  })

  it('requires a note when something is damaged', () => {
    expect(
      validateCheckout({ condition: 'ADA_MASALAH', note: '   ', photo }).note
    ).toBeDefined()
    expect(
      validateCheckout({ condition: 'ADA_MASALAH', note: 'Kursi patah', photo })
    ).toEqual({})
  })

  it('caps the note at 2000 characters', () => {
    expect(
      validateCheckout({ condition: 'NORMAL', note: 'a'.repeat(2001), photo })
        .note
    ).toBeDefined()
  })
})

describe('parseCheckInQr', () => {
  it('reads the room and token from the door QR address', () => {
    expect(
      parseCheckInQr('https://classlab.example/check-in?room=KLS-101&t=abc123')
    ).toEqual({ room: 'KLS-101', token: 'abc123' })
  })

  it('accepts a different origin and a trailing slash', () => {
    expect(
      parseCheckInQr('http://localhost:5173/check-in/?room=LAB-1&t=tok')
    ).toEqual({ room: 'LAB-1', token: 'tok' })
  })

  it('rejects anything that is not a check-in address', () => {
    for (const text of [
      'hello',
      '',
      'https://classlab.example/login?room=A&t=b',
      'https://classlab.example/check-in?room=A',
      'https://classlab.example/check-in?t=b',
      'javascript:alert(1)',
      'https://classlab.example/check-in?room=&t=b',
      `https://classlab.example/check-in?room=A&t=${'x'.repeat(200)}`,
    ]) {
      expect(parseCheckInQr(text), text).toBeNull()
    }
  })
})
