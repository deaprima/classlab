import { describe, expect, it } from 'vitest'
import { parseFacilities, roomFormSchema } from './rooms/room-schema'
import { scheduleFormSchema, sessionCount } from './schedules/schedule-schema'
import { createUserSchema, editUserSchema } from './users/user-schema'

describe('room form', () => {
  const base = {
    code: 'LAB-4',
    name: 'Lab AI',
    type: 'LAB' as const,
    capacity: '30',
    facilities: 'Proyektor, AC\n30 PC\n\n',
    is_active: true,
  }

  it('splits facilities on commas and lines, dropping blanks', () => {
    expect(parseFacilities(base.facilities)).toEqual([
      'Proyektor',
      'AC',
      '30 PC',
    ])
  })

  it('accepts a valid room and rejects bad codes and capacities', () => {
    expect(roomFormSchema.safeParse(base).success).toBe(true)
    expect(roomFormSchema.safeParse({ ...base, code: '-LAB' }).success).toBe(
      false
    )
    expect(roomFormSchema.safeParse({ ...base, code: 'LAB 4' }).success).toBe(
      false
    )
    expect(roomFormSchema.safeParse({ ...base, capacity: '0' }).success).toBe(
      false
    )
    expect(
      roomFormSchema.safeParse({ ...base, capacity: '10001' }).success
    ).toBe(false)
  })

  it('limits facilities to 30 entries', () => {
    const many = Array.from({ length: 31 }, (_, i) => `F${i}`).join(',')
    expect(
      roomFormSchema.safeParse({ ...base, facilities: many }).success
    ).toBe(false)
  })
})

describe('schedule form (BR-09)', () => {
  const base = {
    room_id: 'r1',
    title: 'Basis Data',
    date: '2026-10-19',
    start_time: '07:30',
    end_time: '10:00',
    source: 'AKADEMIK' as const,
    repeat: false,
    until: '',
  }

  it('counts weekly sessions with until inclusive', () => {
    expect(sessionCount('2026-10-19', '2026-10-19')).toBe(1)
    expect(sessionCount('2026-10-19', '2026-10-25')).toBe(1)
    expect(sessionCount('2026-10-19', '2026-10-26')).toBe(2)
    expect(sessionCount('2026-10-19', '2027-04-12')).toBe(26)
    expect(sessionCount('2026-10-19', '2026-10-18')).toBe(0)
  })

  it('allows up to 26 sessions', () => {
    expect(
      scheduleFormSchema.safeParse({
        ...base,
        repeat: true,
        until: '2027-04-12',
      }).success
    ).toBe(true)
    const tooMany = scheduleFormSchema.safeParse({
      ...base,
      repeat: true,
      until: '2027-04-19',
    })
    expect(tooMany.error?.issues[0].message).toBe('Maksimal 26 sesi mingguan.')
  })

  it('requires the end after the start and a valid until when repeating', () => {
    expect(
      scheduleFormSchema.safeParse({ ...base, end_time: '07:00' }).success
    ).toBe(false)
    expect(
      scheduleFormSchema.safeParse({
        ...base,
        repeat: true,
        until: '2026-10-01',
      }).success
    ).toBe(false)
  })
})

describe('user forms', () => {
  it('validates identifiers, email and password length', () => {
    const base = {
      identifier: '2021101006',
      name: 'Fajar',
      email: 'fajar@classlab.test',
      role: 'MAHASISWA' as const,
      password: 'rahasia-123',
      pic_name: '',
      pic_phone: '',
    }
    expect(createUserSchema.safeParse(base).success).toBe(true)
    expect(
      createUserSchema.safeParse({ ...base, identifier: 'ab' }).success
    ).toBe(false)
    expect(
      createUserSchema.safeParse({ ...base, email: 'bukan-email' }).success
    ).toBe(false)
    expect(
      createUserSchema.safeParse({ ...base, password: 'pendek' }).success
    ).toBe(false)
  })

  it('treats email as optional', () => {
    const r = createUserSchema.safeParse({
      identifier: '2021101006',
      name: 'Fajar',
      email: '',
      role: 'MAHASISWA',
      password: 'rahasia-123',
      pic_name: '',
      pic_phone: '',
    })
    expect(r.success).toBe(true)
  })

  it('requires the person in charge only for ORGANISASI', () => {
    const base = {
      identifier: 'himpunan01',
      name: 'Himpunan Informatika',
      email: '',
      role: 'ORGANISASI' as const,
      password: 'rahasia-123',
      pic_name: '',
      pic_phone: '',
    }
    const missing = createUserSchema.safeParse(base)
    expect(missing.error?.issues.map((i) => i.path[0])).toEqual([
      'pic_name',
      'pic_phone',
    ])
    expect(
      createUserSchema.safeParse({
        ...base,
        pic_name: 'Rina',
        pic_phone: '081234567890',
      }).success
    ).toBe(true)
    expect(editUserSchema.safeParse({ ...base, is_active: true }).success).toBe(
      false
    )
  })
})
