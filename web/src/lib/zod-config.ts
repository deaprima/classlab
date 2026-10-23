import { z } from 'zod'

// The Content-Security-Policy has no 'unsafe-eval'. Zod probes for it with Function('') when a schema is
// built, and the browser reports a violation each time; with jitless it never tries and parses the same
// way. Import this module before any module that defines a schema, or the probe has already run.
z.config({ jitless: true })
