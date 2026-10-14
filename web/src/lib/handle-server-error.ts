import { toast } from 'sonner'
import { getErrorMessage } from './api-error'

export function handleServerError(error: unknown) {
  if (import.meta.env.DEV) {
    // eslint-disable-next-line no-console
    console.log(error)
  }

  toast.error(getErrorMessage(error))
}
