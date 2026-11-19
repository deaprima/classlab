import {
  type AvailabilityQuery,
  type BookingListQuery,
  type ImportKind,
  type NotificationListQuery,
  type ReportListQuery,
  type RoomListQuery,
  type UserListQuery,
} from './api-types'

// Structured query keys: invalidating a prefix (e.g. queryKeys.bookings.all) refreshes every list
// and detail under it.
export const queryKeys = {
  auth: {
    me: ['auth', 'me'] as const,
  },
  attachments: {
    letters: ['attachments', 'letters'] as const,
  },
  roomTypes: {
    all: ['room-types'] as const,
  },
  availability: {
    all: ['availability'] as const,
    check: (query: AvailabilityQuery) => ['availability', query] as const,
  },
  rooms: {
    all: ['rooms'] as const,
    list: (query: RoomListQuery) => ['rooms', 'list', query] as const,
    detail: (id: string) => ['rooms', 'detail', id] as const,
    qr: (id: string) => ['rooms', 'qr', id] as const,
    schedule: (id: string, from: string, to: string) =>
      ['rooms', 'schedule', id, { from, to }] as const,
  },
  bookings: {
    all: ['bookings'] as const,
    list: (query: BookingListQuery) => ['bookings', 'list', query] as const,
    detail: (id: string) => ['bookings', 'detail', id] as const,
    events: (id: string) => ['bookings', 'events', id] as const,
  },
  notifications: {
    all: ['notifications'] as const,
    list: (query: NotificationListQuery) =>
      ['notifications', 'list', query] as const,
    unreadCount: ['notifications', 'unread-count'] as const,
  },
  reports: {
    all: ['reports'] as const,
    list: (query: ReportListQuery) => ['reports', 'list', query] as const,
    detail: (id: string) => ['reports', 'detail', id] as const,
    photo: (id: string) => ['reports', 'photo', id] as const,
  },
  imports: {
    all: ['imports'] as const,
    list: (query: { kind?: ImportKind; page?: number; per_page?: number }) =>
      ['imports', 'list', query] as const,
  },
  users: {
    all: ['users'] as const,
    violations: (id: string) => ['users', 'violations', id] as const,
    list: (query: UserListQuery) => ['users', 'list', query] as const,
  },
}
