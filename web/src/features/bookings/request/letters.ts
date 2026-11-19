import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/lib/api-client'
import { type Attachment } from '@/lib/api-types'
import { queryKeys } from '@/lib/query-keys'

/** Letters this user uploaded before, newest first, so one can be attached again (BR-19). */
export function useLetters(enabled: boolean) {
  return useQuery({
    queryKey: queryKeys.attachments.letters,
    queryFn: async () =>
      (
        await api.get<{ data: Attachment[] }>('/attachments', {
          params: { kind: 'LETTER', per_page: 20 },
        })
      ).data.data,
    enabled,
  })
}

/** Uploads a letter; the server inspects the file itself, so any 422 comes back on the `file` field. */
export function useUploadLetter() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (file: File) => {
      const body = new FormData()
      body.set('kind', 'LETTER')
      body.set('file', file)
      return (await api.post<Attachment>('/attachments', body)).data
    },
    onSuccess: () =>
      queryClient.invalidateQueries({
        queryKey: queryKeys.attachments.letters,
      }),
  })
}
