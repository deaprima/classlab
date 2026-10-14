import {
  type AvailabilityQuery,
  type BookingListQuery,
  type RoomListQuery,
} from './api-types'

// Structured query keys: invalidating a prefix (e.g. queryKeys.bookings.all) refreshes every list
// and detail under it.
export const queryKeys = {
  availability: {
    all: ['availability'] as const,
    check: (query: AvailabilityQuery) => ['availability', query] as const,
  },
  rooms: {
    all: ['rooms'] as const,
    list: (query: RoomListQuery) => ['rooms', 'list', query] as const,
    detail: (id: string) => ['rooms', 'detail', id] as const,
    schedule: (id: string, from: string, to: string) =>
      ['rooms', 'schedule', id, { from, to }] as const,
  },
  bookings: {
    all: ['bookings'] as const,
    list: (query: BookingListQuery) => ['bookings', 'list', query] as const,
    detail: (id: string) => ['bookings', 'detail', id] as const,
    events: (id: string) => ['bookings', 'events', id] as const,
  },
}
