import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import { toast } from 'sonner'
import { api } from '@/lib/api-client'
import {
  type DamageReport,
  type ReportDetail,
  type ReportListQuery,
  type ReportListResponse,
  type ReportStatus,
} from '@/lib/api-types'
import { queryKeys } from '@/lib/query-keys'
import { REPORT_STATUS_LABELS } from '@/lib/status-labels'
import { useDataUrl } from '@/hooks/use-data-url'

export function useReports(query: ReportListQuery) {
  return useQuery({
    queryKey: queryKeys.reports.list(query),
    queryFn: async () =>
      (await api.get<ReportListResponse>('/reports', { params: query })).data,
    placeholderData: keepPreviousData,
  })
}

export function useReport(id: string | undefined) {
  return useQuery({
    queryKey: queryKeys.reports.detail(id ?? ''),
    queryFn: async () => (await api.get<ReportDetail>(`/reports/${id}`)).data,
    enabled: !!id,
  })
}

/**
 * The photo lives outside the webroot and needs the bearer token (decisions.md, Phase 4), so an
 * <img src> cannot load it directly. It is fetched as a blob and shown as a data URL.
 */
export function useReportPhoto(
  report: Pick<DamageReport, 'id' | 'photo_path'> | undefined
) {
  const id = report?.photo_path ? report.id : undefined
  const blob = useQuery({
    queryKey: queryKeys.reports.photo(id ?? ''),
    queryFn: async () =>
      (await api.get<Blob>(`/reports/${id}/photo`, { responseType: 'blob' }))
        .data,
    enabled: !!id,
    staleTime: Infinity,
  })

  const url = useDataUrl(blob.data)

  return {
    url,
    isLoading: blob.isLoading,
    error: blob.error,
  }
}

export type CreateReportInput = {
  room_id: string
  description: string
  photo?: File
}

export function useCreateReport(options: {
  onSuccess?: (report: DamageReport) => void
  onError?: (error: unknown) => void
}) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (input: CreateReportInput) => {
      const form = new FormData()
      form.append('room_id', input.room_id)
      form.append('description', input.description)
      if (input.photo) form.append('photo', input.photo)
      // axios sets the multipart boundary itself for FormData.
      return (await api.post<DamageReport>('/reports', form)).data
    },
    onSuccess: (report) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.reports.all })
      options.onSuccess?.(report)
    },
    onError: options.onError,
  })
}

export function useUpdateReportStatus() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (input: {
      id: string
      status: ReportStatus
      note?: string
    }) =>
      (
        await api.patch<DamageReport>(`/reports/${input.id}/status`, {
          status: input.status,
          ...(input.note && { note: input.note }),
        })
      ).data,
    onSuccess: (report) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.reports.all })
      toast.success(
        `Status laporan menjadi ${REPORT_STATUS_LABELS[report.status]}.`
      )
    },
  })
}
