import { useRef } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import { type Booking, type CreateBookingRequest } from '@/lib/api-types'
import { queryKeys } from '@/lib/query-keys'

type CreateBookingVars = { body: CreateBookingRequest; idempotencyKey: string }

export function useCreateBooking(options: {
  onSuccess?: (booking: Booking) => void
  onError?: (error: unknown) => void
}) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ body, idempotencyKey }: CreateBookingVars) =>
      (
        await api.post<Booking>('/bookings', body, {
          headers: { 'Idempotency-Key': idempotencyKey },
        })
      ).data,
    onSuccess: (booking) => {
      // A new pending booking changes availability and every booking list.
      queryClient.invalidateQueries({ queryKey: queryKeys.bookings.all })
      queryClient.invalidateQueries({ queryKey: queryKeys.availability.all })
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
