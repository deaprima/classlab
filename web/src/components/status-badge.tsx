import {
  type AvailabilityStatus,
  type BookingStatus,
  type ReportStatus,
} from '@/lib/api-types'
import {
  AVAILABILITY_LABELS,
  BOOKING_STATUS_LABELS,
  REPORT_STATUS_LABELS,
} from '@/lib/status-labels'
import {
  AVAILABILITY_PILL,
  BOOKING_PILL,
  REPORT_PILL,
} from '@/lib/status-pill-map'
import { StatusPill } from './status-pill'

export function AvailabilityBadge({ status }: { status: AvailabilityStatus }) {
  return (
    <StatusPill
      status={AVAILABILITY_PILL[status]}
      label={AVAILABILITY_LABELS[status]}
    />
  )
}

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
