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
import { cn } from '@/lib/utils'

type Tone = 'green' | 'amber' | 'red' | 'blue' | 'gray'

const toneClass: Record<Tone, string> = {
  green:
    'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950 dark:text-emerald-300',
  amber:
    'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-300',
  red: 'border-red-200 bg-red-50 text-red-800 dark:border-red-900 dark:bg-red-950 dark:text-red-300',
  blue: 'border-sky-200 bg-sky-50 text-sky-800 dark:border-sky-900 dark:bg-sky-950 dark:text-sky-300',
  gray: 'border-border bg-muted text-muted-foreground',
}

function Pill({ tone, label }: { tone: Tone; label: string }) {
  return (
    <span
      className={cn(
        'inline-flex w-fit shrink-0 items-center rounded-md border px-2 py-0.5 text-xs font-medium whitespace-nowrap',
        toneClass[tone]
      )}
    >
      {label}
    </span>
  )
}

const availabilityTone: Record<AvailabilityStatus, Tone> = {
  TERSEDIA: 'green',
  DIAJUKAN: 'amber',
  DIGUNAKAN: 'red',
}

export function AvailabilityBadge({ status }: { status: AvailabilityStatus }) {
  return (
    <Pill tone={availabilityTone[status]} label={AVAILABILITY_LABELS[status]} />
  )
}

const bookingTone: Record<BookingStatus, Tone> = {
  MENUNGGU: 'amber',
  DISETUJUI: 'green',
  DITOLAK: 'red',
  DIBATALKAN: 'gray',
}

export function BookingStatusBadge({ status }: { status: BookingStatus }) {
  return (
    <Pill tone={bookingTone[status]} label={BOOKING_STATUS_LABELS[status]} />
  )
}

const reportTone: Record<ReportStatus, Tone> = {
  BARU: 'blue',
  DITERIMA: 'amber',
  DIPROSES: 'amber',
  SELESAI: 'green',
}

export function ReportStatusBadge({ status }: { status: ReportStatus }) {
  return <Pill tone={reportTone[status]} label={REPORT_STATUS_LABELS[status]} />
}
