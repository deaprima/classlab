import {
  AxiosError,
  AxiosHeaders,
  type AxiosAdapter,
  type AxiosResponse,
  type InternalAxiosRequestConfig,
} from 'axios'

type MockReply = { status: number; data?: unknown }
type Handler = (
  config: InternalAxiosRequestConfig
) => MockReply | Promise<MockReply>

/** Axios adapter that answers from `handler` instead of the network, with axios' own error semantics. */
export function mockAdapter(handler: Handler): AxiosAdapter {
  return async (config) => {
    const { status, data } = await handler(config)
    const response: AxiosResponse = {
      data,
      status,
      statusText: String(status),
      headers: new AxiosHeaders(),
      config,
      request: {},
    }
    if (status >= 200 && status < 300) return response
    throw new AxiosError(
      `Request failed with status code ${status}`,
      status >= 500 ? AxiosError.ERR_BAD_RESPONSE : AxiosError.ERR_BAD_REQUEST,
      config,
      {},
      response
    )
  }
}

export function apiErrorBody(
  code: string,
  message: string,
  details: Record<string, string>[] = []
) {
  return { error: { code, message, details, request_id: 'req_test' } }
}

export const sampleUser = {
  id: '018f23a4-8f0b-789a-bcde-0123456789ab',
  org_unit_id: '018f23a4-8f0b-789a-bcde-000000000001',
  identifier: '220101001',
  name: 'Budi Santoso',
  email: 'budi@student.univ.ac.id',
  role: 'MAHASISWA',
  is_active: true,
  booking_blocked_until: null,
  pic_name: null,
  pic_phone: null,
  must_change_password: false,
  created_at: '2026-10-01T08:00:00+07:00',
} as const
