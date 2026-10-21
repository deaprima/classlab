import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import {
  type CheckInResult,
  type CheckOutResult,
  type CheckoutCondition,
} from '@/lib/api-types'
import { queryKeys } from '@/lib/query-keys'

/** Check-in and check-out change a booking's status and, for check-out, free the rest of the slot. */
function useRefreshAfterAttendance() {
  const queryClient = useQueryClient()
  return () => {
    queryClient.invalidateQueries({ queryKey: queryKeys.bookings.all })
    queryClient.invalidateQueries({ queryKey: queryKeys.availability.all })
    queryClient.invalidateQueries({ queryKey: queryKeys.rooms.all })
    queryClient.invalidateQueries({ queryKey: queryKeys.reports.all })
    queryClient.invalidateQueries({ queryKey: queryKeys.notifications.all })
  }
}

type CheckInInput = { room_code: string; qr_token: string }

export function useCheckIn() {
  const refresh = useRefreshAfterAttendance()
  return useMutation({
    mutationFn: async (input: CheckInInput) =>
      (await api.post<CheckInResult>('/check-in', input)).data,
    onSuccess: refresh,
  })
}

export type CheckOutInput = {
  bookingId: string
  condition: CheckoutCondition
  note?: string
  photo: Blob
}

export function useCheckOut() {
  const refresh = useRefreshAfterAttendance()
  return useMutation({
    mutationFn: async ({
      bookingId,
      condition,
      note,
      photo,
    }: CheckOutInput) => {
      const form = new FormData()
      form.append('condition', condition)
      if (note) form.append('note', note)
      // The server re-encodes the photo; the file name only has to look like an image.
      form.append('photo', photo, 'checkout.jpg')
      // axios sets the multipart boundary itself for FormData.
      return (
        await api.post<CheckOutResult>(`/bookings/${bookingId}/check-out`, form)
      ).data
    },
    onSuccess: refresh,
  })
}
