/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** API base path; defaults to `/api/v1` (same origin, proxied by Vite in dev and Caddy in prod). */
  readonly VITE_API_BASE_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
