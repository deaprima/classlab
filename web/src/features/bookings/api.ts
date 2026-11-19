import { useRef } from 'react'
import {
  keepPreviousData,
  type QueryClient,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import { toast } from 'sonner'
import { api } from '@/lib/api-client'
import { getApiError, getErrorMessage } from '@/lib/api-error'
import {
  type Booking,
  type BookingEvent,
  type BookingGroupResult,
  type BookingListQuery,
  type BookingListResponse,
  type CreateBookingRequest,
  type EditBookingRequest,
} from '@/lib/api-types'
import { formatSlot } from '@/lib/datetime'
import { queryKeys } from '@/lib/query-keys'

export function useBookings(query: BookingListQuery, enabled = true) {
  return useQuery({
    queryKey: queryKeys.bookings.list(query),
    enabled,
    queryFn: async () =>
      (await api.get<BookingListResponse>('/bookings', { params: query })).data,
    placeholderData: keepPreviousData,
  })
}

export function useBooking(id: string | undefined) {
  return useQuery({
    queryKey: queryKeys.bookings.detail(id ?? ''),
    queryFn: async () => (await api.get<Booking>(`/bookings/${id}`)).data,
    enabled: !!id,
  })
}

export function useBookingEvents(id: string | undefined) {
  return useQuery({
    queryKey: queryKeys.bookings.events(id ?? ''),
    queryFn: async () =>
      (await api.get<{ data: BookingEvent[] }>(`/bookings/${id}/events`)).data
        .data,
    enabled: !!id,
  })
}

/** Every status change moves slots and lists: availability, timelines and all booking queries. */
function invalidateAfterStatusChange(queryClient: QueryClient) {
  queryClient.invalidateQueries({ queryKey: queryKeys.bookings.all })
  queryClient.invalidateQueries({ queryKey: queryKeys.availability.all })
  queryClient.invalidateQueries({ queryKey: queryKeys.rooms.all })
}

type CreateBookingVars = { body: CreateBookingRequest; idempotencyKey: string }

export function useCreateBooking(options: {
  onSuccess?: (result: Booking | BookingGroupResult) => void
  onError?: (error: unknown) => void
}) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ body, idempotencyKey }: CreateBookingVars) =>
      (
        await api.post<Booking | BookingGroupResult>('/bookings', body, {
          headers: { 'Idempotency-Key': idempotencyKey },
        })
      ).data,
    onSuccess: (result) => {
      invalidateAfterStatusChange(queryClient)
      options.onSuccess?.(result)
    },
    onError: options.onError,
  })
}

/**
 * Status changes report failure as a toast. A 409 conflict on approval also lists what now occupies
 * the slot, since that is the one case the approver cannot see coming from the queue.
 */
function toastDecisionError(error: unknown) {
  const apiError = getApiError(error)
  if (apiError?.code === 'BOOKING_CONFLICT' && apiError.details.length > 0) {
    toast.error(apiError.message, {
      description: apiError.details
        .flatMap((d) =>
          d.start_at && d.end_at
            ? [`Bentrok: ${formatSlot(d.start_at, d.end_at)}`]
            : []
        )
        .join('\n'),
    })
    return
  }
  toast.error(getErrorMessage(error))
}

export function useApproveBooking() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) =>
      (await api.post<Booking>(`/bookings/${id}/approve`)).data,
    onSuccess: (booking) => {
      invalidateAfterStatusChange(queryClient)
      toast.success(`Pengajuan ${booking.requester.name} disetujui.`)
    },
    onError: toastDecisionError,
  })
}

export function useRejectBooking() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, reason }: { id: string; reason: string }) =>
      (await api.post<Booking>(`/bookings/${id}/reject`, { reason })).data,
    onSuccess: (booking) => {
      invalidateAfterStatusChange(queryClient)
      toast.success(`Pengajuan ${booking.requester.name} ditolak.`)
    },
    onError: toastDecisionError,
  })
}

export type CancelScope = 'OCCURRENCE' | 'GROUP'

export function useCancelBooking() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({
      id,
      reason,
      scope,
    }: {
      id: string
      reason?: string
      /** A multi-date request: just this date (default) or every date still cancellable (BR-27). */
      scope?: CancelScope
    }) =>
      (
        await api.post<Booking>(`/bookings/${id}/cancel`, {
          ...(reason && { reason }),
          ...(scope && { scope }),
        })
      ).data,
    onSuccess: () => {
      invalidateAfterStatusChange(queryClient)
      toast.success('Peminjaman dibatalkan.')
    },
    onError: toastDecisionError,
  })
}

/** The owner changes a waiting or approved booking (BR-26); only the fields sent are changed. */
export function useEditBooking(options: {
  onSuccess?: (booking: Booking) => void
  onError?: (error: unknown) => void
}) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({
      id,
      body,
    }: {
      id: string
      body: EditBookingRequest
    }) => (await api.patch<Booking>(`/bookings/${id}`, body)).data,
    onSuccess: (booking) => {
      invalidateAfterStatusChange(queryClient)
      options.onSuccess?.(booking)
    },
    onError: options.onError,
  })
}

/**
 * One Idempotency-Key per distinct request body: resubmitting the same booking (double click, retry
 * after a timeout) reuses the key so the server returns the original booking instead of a second
 * one or a false 409; editing the form gets a new key, since the API rejects a reused key with a
 * different body (decisions.md, Phase 3).
 */
export function useIdempotencyKey() {
  const last = useRef<{ payload: string; key: string } | null>(null)
  return (body: unknown): string => {
    const payload = JSON.stringify(body)
    if (last.current?.payload !== payload) {
      last.current = { payload, key: crypto.randomUUID() }
    }
    return last.current.key
  }
}
