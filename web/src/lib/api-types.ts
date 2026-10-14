import { type components, type paths } from '@/types/api'

// Short names for the generated OpenAPI types. Never hand-write response shapes; add an alias here instead.
type Schemas = components['schemas']

export type User = Schemas['User']
export type UserRole = Schemas['UserRole']
export type ErrorBody = Schemas['ErrorBody']

export type LoginRequest =
  paths['/auth/login']['post']['requestBody']['content']['application/json']
export type LoginResponse =
  paths['/auth/login']['post']['responses']['200']['content']['application/json']
export type RefreshResponse =
  paths['/auth/refresh']['post']['responses']['200']['content']['application/json']
