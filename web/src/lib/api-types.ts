import { type components, type paths } from '@/types/api'

// Short names for the generated OpenAPI types. Never hand-write response shapes; add an alias here instead.
type Schemas = components['schemas']

export type User = Schemas['User']
export type UserRole = Schemas['UserRole']
export type ErrorBody = Schemas['ErrorBody']
export type PaginationMeta = Schemas['PaginationMeta']

export type Room = Schemas['Room']
export type RoomType = Schemas['RoomType']
export type TimelineItem = Schemas['TimelineItem']
export type AvailabilityRoom = Schemas['AvailabilityRoom']
export type AvailabilityStatus = AvailabilityRoom['status']
export type AvailabilityConflict = AvailabilityRoom['conflicts'][number]

export type Booking = Schemas['Booking']
export type BookingStatus = Schemas['BookingStatus']

export type LoginRequest =
  paths['/auth/login']['post']['requestBody']['content']['application/json']
export type LoginResponse =
  paths['/auth/login']['post']['responses']['200']['content']['application/json']
export type RefreshResponse =
  paths['/auth/refresh']['post']['responses']['200']['content']['application/json']

export type RoomListQuery = NonNullable<
  paths['/rooms']['get']['parameters']['query']
>
export type RoomListResponse =
  paths['/rooms']['get']['responses']['200']['content']['application/json']
export type RoomScheduleResponse =
  paths['/rooms/{id}/schedule']['get']['responses']['200']['content']['application/json']

export type AvailabilityQuery =
  paths['/availability']['get']['parameters']['query']
export type AvailabilityResponse =
  paths['/availability']['get']['responses']['200']['content']['application/json']

export type CreateBookingRequest =
  paths['/bookings']['post']['requestBody']['content']['application/json']
