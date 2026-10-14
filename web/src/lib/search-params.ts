import { z } from 'zod'

// Reusable URL search param validators. `.catch(undefined)` drops a malformed value instead of
// failing the route, so a hand-edited URL still opens the page.
export const dateParam = z
  .string()
  .regex(/^\d{4}-\d{2}-\d{2}$/)
  .optional()
  .catch(undefined)

export const timeParam = z
  .string()
  .regex(/^([01]\d|2[0-3]):[0-5]\d$/)
  .optional()
  .catch(undefined)

export const uuidParam = z.uuid().optional().catch(undefined)

export const positiveIntParam = z
  .number()
  .int()
  .positive()
  .optional()
  .catch(undefined)
