import { useMutation } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api } from '@/lib/api-client'
import { getErrorMessage } from '@/lib/api-error'

/**
 * The letter is behind the bearer token (decisions.md, Phase 3), so a plain link cannot open it:
 * it is fetched as a blob and saved from an object URL that is revoked right after.
 */
export function useDownloadLetter() {
  return useMutation({
    mutationFn: async ({ id, name }: { id: string; name: string }) => {
      const { data } = await api.get<Blob>(`/attachments/${id}`, {
        responseType: 'blob',
      })
      const url = URL.createObjectURL(data)
      const link = document.createElement('a')
      link.href = url
      link.download = name
      link.click()
      URL.revokeObjectURL(url)
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  })
}
