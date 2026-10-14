import { AxiosError, AxiosHeaders } from 'axios'
import { apiErrorBody } from '@/test-utils/mock-adapter'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { handleServerError } from './handle-server-error'

const toastError = vi.hoisted(() => vi.fn())

vi.mock('sonner', () => ({
  toast: {
    error: toastError,
  },
}))

beforeEach(() => {
  vi.mocked(toastError).mockClear()
})

describe('handleServerError', () => {
  it('shows the message from the API error body', () => {
    const config = { headers: new AxiosHeaders() }
    const error = new AxiosError(
      'failed',
      'ERR_BAD_REQUEST',
      config,
      {},
      {
        status: 409,
        statusText: '',
        headers: {},
        config,
        data: apiErrorBody('INVALID_STATE', 'Pengajuan sudah diputuskan.'),
      }
    )

    handleServerError(error)

    expect(toastError).toHaveBeenCalledWith('Pengajuan sudah diputuskan.')
  })

  it('shows a fallback for errors that did not come from the API', () => {
    const log = vi.spyOn(console, 'log').mockImplementation(() => {})

    handleServerError(new Error('boom'))

    expect(toastError).toHaveBeenCalledWith(
      'Permintaan tidak dapat diproses. Coba lagi.'
    )
    log.mockRestore()
  })

  it('logs the error to the console in development', () => {
    const log = vi.spyOn(console, 'log').mockImplementation(() => {})
    const err = new Error('logged')

    handleServerError(err)

    expect(log).toHaveBeenCalledWith(err)
    log.mockRestore()
  })

  it('does not log the error to the console in production', () => {
    vi.stubEnv('DEV', false)
    const log = vi.spyOn(console, 'log').mockImplementation(() => {})

    handleServerError(new Error('not logged'))

    expect(log).not.toHaveBeenCalled()
    log.mockRestore()
  })
})
