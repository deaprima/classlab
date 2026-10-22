import { describe, expect, it } from 'vitest'
import {
  nextReportStatus,
  remainingSteps,
  reportFormSchema,
} from './report-rules'

const file = (type: string, size: number) =>
  new File([new Uint8Array(size)], 'foto', { type })

describe('nextReportStatus (BR-08)', () => {
  it('moves one step forward and stops at SELESAI', () => {
    expect(nextReportStatus('BARU')).toBe('DITERIMA')
    expect(nextReportStatus('DITERIMA')).toBe('DIPROSES')
    expect(nextReportStatus('DIPROSES')).toBe('SELESAI')
    expect(nextReportStatus('SELESAI')).toBeNull()
  })
})

describe('reportFormSchema', () => {
  const base = { room_id: 'r1', description: 'Proyektor mati' }

  it('accepts a report without a photo', () => {
    expect(reportFormSchema.safeParse(base).success).toBe(true)
  })

  it('accepts JPG/PNG/WebP up to 5 MB', () => {
    for (const type of ['image/jpeg', 'image/png', 'image/webp']) {
      expect(
        reportFormSchema.safeParse({ ...base, photo: file(type, 1024) }).success
      ).toBe(true)
    }
  })

  it('rejects other types and files over 5 MB', () => {
    const gif = reportFormSchema.safeParse({
      ...base,
      photo: file('image/gif', 10),
    })
    expect(gif.error?.issues[0].message).toBe(
      'Foto harus berformat JPG, PNG, atau WebP.'
    )
    const big = reportFormSchema.safeParse({
      ...base,
      photo: file('image/jpeg', 5 * 1024 * 1024 + 1),
    })
    expect(big.error?.issues[0].message).toBe('Ukuran foto maksimal 5 MB.')
  })

  it('requires a description', () => {
    expect(
      reportFormSchema.safeParse({ ...base, description: '  ' }).error
        ?.issues[0].message
    ).toBe('Jelaskan kerusakannya.')
  })
})

describe('remainingSteps', () => {
  it('lists the steps still to come, none once finished', () => {
    expect(remainingSteps('BARU')).toEqual(['DITERIMA', 'DIPROSES', 'SELESAI'])
    expect(remainingSteps('DIPROSES')).toEqual(['SELESAI'])
    expect(remainingSteps('SELESAI')).toEqual([])
  })
})
