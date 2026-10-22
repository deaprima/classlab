import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import { type Checkout, type PaginationMeta } from '@/lib/api-types'
import { useDataUrl } from '@/hooks/use-data-url'

type Page<T> = { data: T[]; meta: PaginationMeta }

type CheckoutQuery = {
  from: string
  to: string
  per_page: number
  room_id?: string
}

/** Checkouts in [from, to), newest first (OB, TU, KALAB, ADMIN; the server scopes TU and KALAB to their types). */
export function useCheckouts(query: CheckoutQuery) {
  return useQuery({
    queryKey: ['checkouts', 'list', query] as const,
    queryFn: async () =>
      (await api.get<Page<Checkout>>('/checkouts', { params: query })).data,
    placeholderData: keepPreviousData,
  })
}

/**
 * The photo of how a room was left. It sits behind the bearer token, so it is fetched as a blob.
 * Photos are deleted by the retention policy; the API then answers 410 and `error` says so.
 */
export function useCheckoutPhoto(attachmentId: string) {
  const blob = useQuery({
    queryKey: ['attachments', 'checkout', attachmentId] as const,
    queryFn: async () =>
      (
        await api.get<Blob>(`/attachments/${attachmentId}`, {
          responseType: 'blob',
        })
      ).data,
    staleTime: Infinity,
    retry: false,
  })
  return {
    url: useDataUrl(blob.data),
    isLoading: blob.isLoading,
    error: blob.error,
  }
}
