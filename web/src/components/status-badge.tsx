import { type BookingStatus, type ReportStatus } from '@/lib/api-types'
import {
  BOOKING_STATUS_LABELS,
  REPORT_STATUS_LABELS,
} from '@/lib/status-labels'
import { BOOKING_PILL, REPORT_PILL } from '@/lib/status-pill-map'
import { StatusPill } from './status-pill'

export function BookingStatusBadge({ status }: { status: BookingStatus }) {
  return (
    <StatusPill
      status={BOOKING_PILL[status]}
      label={BOOKING_STATUS_LABELS[status]}
    />
  )
}

export function ReportStatusBadge({ status }: { status: ReportStatus }) {
  return (
    <StatusPill
      status={REPORT_PILL[status]}
      label={REPORT_STATUS_LABELS[status]}
    />
  )
}
