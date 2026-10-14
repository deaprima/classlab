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

export type BookingEvent = Schemas['BookingEvent']
export type BookingListQuery = NonNullable<
  paths['/bookings']['get']['parameters']['query']
>
export type BookingListResponse =
  paths['/bookings']['get']['responses']['200']['content']['application/json']

// "Notification" would shadow the DOM type of the same name.
export type AppNotification = Schemas['Notification']
export type NotificationListQuery = NonNullable<
  paths['/notifications']['get']['parameters']['query']
>
export type NotificationListResponse =
  paths['/notifications']['get']['responses']['200']['content']['application/json']

export type DamageReport = Schemas['DamageReport']
export type ReportStatus = Schemas['ReportStatus']
export type ReportEvent = Schemas['ReportEvent']
export type ReportListQuery = NonNullable<
  paths['/reports']['get']['parameters']['query']
>
export type ReportListResponse =
  paths['/reports']['get']['responses']['200']['content']['application/json']
export type ReportDetail =
  paths['/reports/{id}']['get']['responses']['200']['content']['application/json']

export type Schedule = Schemas['Schedule']
export type ScheduleSource = Schemas['ScheduleSource']
export type CreateRoomRequest =
  paths['/rooms']['post']['requestBody']['content']['application/json']
export type UpdateRoomRequest =
  paths['/rooms/{id}']['patch']['requestBody']['content']['application/json']
export type CreateScheduleRequest =
  paths['/schedules']['post']['requestBody']['content']['application/json']
export type UpdateScheduleRequest =
  paths['/schedules/{id}']['patch']['requestBody']['content']['application/json']
export type UserListQuery = NonNullable<
  paths['/users']['get']['parameters']['query']
>
export type UserListResponse =
  paths['/users']['get']['responses']['200']['content']['application/json']
export type CreateUserRequest =
  paths['/users']['post']['requestBody']['content']['application/json']
export type UpdateUserRequest =
  paths['/users/{id}']['patch']['requestBody']['content']['application/json']
