/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** API base path; defaults to `/api/v1` (same origin, proxied by Vite in dev and Caddy in prod). */
  readonly VITE_API_BASE_URL?: string
  /** `true` shows the TanStack Router and Query devtools in dev; they stay hidden otherwise. */
  readonly VITE_SHOW_DEVTOOLS?: string
  // BR-01 mirrors of the API's env, used only for early form validation (see config/booking-rules.ts).
  readonly VITE_OPERATING_HOURS_START?: string
  readonly VITE_OPERATING_HOURS_END?: string
  readonly VITE_BOOKING_MIN_DURATION_MINUTES?: string
  readonly VITE_BOOKING_MAX_DURATION_HOURS?: string
  readonly VITE_BOOKING_MAX_FUTURE_DAYS?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
