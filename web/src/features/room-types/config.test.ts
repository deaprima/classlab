import { describe, expect, it } from 'vitest'
import { type RoomType } from '@/lib/api-types'
import { ROOM_TYPE_CONFIG, roomTypeConfig, roomTypeFromSlug } from './config'

const ALL_TYPES: RoomType[] = [
  'KELAS',
  'LAB',
  'AUDITORIUM',
  'RUANG_UJIAN',
  'RUANG_SIDANG',
]

describe('room type config', () => {
  it('covers every room type of the API exactly once', () => {
    expect(ROOM_TYPE_CONFIG.map((c) => c.type).sort()).toEqual(
      [...ALL_TYPES].sort()
    )
  })

  it('uses unique URL slugs', () => {
    const slugs = ROOM_TYPE_CONFIG.map((c) => c.slug)
    expect(new Set(slugs).size).toBe(slugs.length)
  })

  it('maps between slug and type', () => {
    expect(roomTypeFromSlug('ujian')).toBe('RUANG_UJIAN')
    expect(roomTypeFromSlug('sidang')).toBe('RUANG_SIDANG')
    expect(roomTypeFromSlug('tidak-ada')).toBeUndefined()
    expect(roomTypeConfig('LAB').slug).toBe('lab')
  })
})
