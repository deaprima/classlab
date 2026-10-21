import { useEffect, useRef, useState } from 'react'
import { useForm, useWatch } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { attendanceRules } from '@/config/attendance-rules'
import { AlertCircle, Loader2 } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { formProblemFrom, getApiError, getErrorMessage } from '@/lib/api-error'
import {
  type Booking,
  type BookingGroupResult,
  type Room,
} from '@/lib/api-types'
import { formatDate, formatDateShort, toWibDate } from '@/lib/datetime'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { type ConflictItem } from '@/components/conflict-list'
import { useMe } from '@/features/auth/api'
import { useRoomTypes } from '@/features/room-types/api'
import { roomTypeConfig } from '@/features/room-types/config'
import { useCreateBooking, useIdempotencyKey } from '../api'
import { LetterField } from './letter-field'
import { OccurrenceFields } from './occurrence-fields'
import {
  approverLabel,
  effectiveHours,
  letterRequired,
  participantsRequired,
  previewOutcome,
} from './policy'
import {
  createRequestSchema,
  emptyOccurrence,
  EMPTY_REQUEST,
  lastBookableDate,
  NOTES_MAX,
  PURPOSE_MAX,
  REQUEST_FIELD_MAP,
  toCreateBookingRequest,
  type OccurrenceValues,
  type RequestContext,
  type RequestFormValues,
} from './request-schema'
import { RequestSummary } from './request-summary'
import { useSlotCheck } from './slot-checks'

type Problem =
  | { kind: 'conflict'; message: string; items: ConflictItem[] }
  | { kind: 'quota'; message: string }
  | { kind: 'blocked'; until: string | undefined }
  | { kind: 'other'; message: string }

type RequestFormProps = {
  /** The room picked on the schedule; the request is for this room. */
  room: Room
  /** The slots picked on the schedule, one per date. Empty: one blank row. */
  initialOccurrences: readonly OccurrenceValues[]
  onCreated: (result: Booking | BookingGroupResult) => void
  onCancel: () => void
}

/** The content of the request popup (docs/DESIGN.md "Ajukan Ruang"): details, summary and actions. */
export function RequestForm({
  room,
  initialOccurrences,
  onCreated,
  onCancel,
}: RequestFormProps) {
  const user = useAuthStore((state) => state.auth.user)
  const me = useMe()
  const policies = useRoomTypes()
  const keyFor = useIdempotencyKey()
  const [problem, setProblem] = useState<Problem | null>(null)
  const role = user?.role ?? 'MAHASISWA'
  const policy = policies.data?.find((p) => p.type === room.type)

  // The schema depends on the policy, which loads after the form exists: the resolver reads it lazily.
  const contextRef = useRef<RequestContext>({ role, room })
  useEffect(() => {
    contextRef.current = { role, room, policy }
  })
  const form = useForm<RequestFormValues>({
    resolver: (values, context, options) =>
      zodResolver(createRequestSchema(contextRef.current))(
        values,
        context,
        options
      ),
    defaultValues: {
      ...EMPTY_REQUEST,
      room_id: room.id,
      occurrences:
        initialOccurrences.length > 0
          ? initialOccurrences.map((o) => ({ ...o }))
          : [emptyOccurrence()],
    },
  })

  const occurrences = useWatch({ control: form.control, name: 'occurrences' })
  const participants = useWatch({ control: form.control, name: 'participants' })
  const slot = useSlotCheck(room.id, occurrences)

  const blockedUntil =
    problem?.kind === 'blocked'
      ? problem.until
      : (me.data?.booking_blocked_until ?? undefined)
  const blocked = !!blockedUntil && new Date(blockedUntil) > new Date()

  const create = useCreateBooking({
    onSuccess: onCreated,
    onError: (error) => {
      const apiError = getApiError(error)
      if (apiError?.code === 'BOOKING_BLOCKED') {
        setProblem({ kind: 'blocked', until: apiError.details[0]?.message })
        return
      }
      if (apiError?.code === 'QUOTA_EXCEEDED') {
        setProblem({ kind: 'quota', message: apiError.message })
        return
      }
      const found = formProblemFrom(error, form.setError, REQUEST_FIELD_MAP)
      if (found?.conflicts) {
        setProblem({
          kind: 'conflict',
          message: found.message,
          items: found.conflicts,
        })
      } else if (found) {
        setProblem({ kind: 'other', message: found.message })
      }
    },
  })

  function onSubmit(values: RequestFormValues) {
    setProblem(null)
    const body = toCreateBookingRequest(values)
    create.mutate({ body, idempotencyKey: keyFor(body) })
  }

  if (policies.isPending) return <Skeleton className='h-96 rounded-2xl' />
  if (policies.isError || !policy) {
    return (
      <Alert variant='error'>
        <AlertCircle />
        <AlertDescription>
          {policies.isError
            ? getErrorMessage(policies.error)
            : 'Kebijakan tipe ruang ini tidak ditemukan.'}
        </AlertDescription>
      </Alert>
    )
  }

  const now = new Date()
  const hours = effectiveHours(room, policy)
  const lastDate = lastBookableDate(policy, now)
  const needLetter = letterRequired(policy, role)
  const outcome =
    problem?.kind === 'conflict'
      ? {
          kind: 'conflict' as const,
          title: problem.message,
          description: 'Pilih jam lain, lalu kirim ulang.',
        }
      : previewOutcome({
          policy,
          role,
          blocked,
          slotFree: slot.free,
          approverLabel: approverLabel(policy),
        })
  const conflicts: ConflictItem[] =
    problem?.kind === 'conflict' ? problem.items : slot.conflicts
  const cannotSubmit =
    create.isPending ||
    blocked ||
    outcome.kind === 'conflict' ||
    problem?.kind === 'conflict'

  return (
    <div className='flex min-w-0 flex-col gap-5'>
      {blocked && blockedUntil && (
        <Alert variant='error'>
          <AlertCircle />
          <AlertTitle>
            Booking baru diblokir sampai {formatDate(blockedUntil)}
          </AlertTitle>
          <AlertDescription>
            Poin pelanggaran aktif mencapai{' '}
            {attendanceRules.blockThresholdPoints}. Booking yang sudah disetujui
            tetap berlaku. Hubungi Admin bila ada keliruan.
          </AlertDescription>
        </Alert>
      )}
      {problem?.kind === 'quota' && (
        <Alert variant='error'>
          <AlertCircle />
          <AlertTitle>Kuota booking aktif penuh</AlertTitle>
          <AlertDescription>{problem.message}</AlertDescription>
        </Alert>
      )}

      <Form {...form}>
        <form
          id='booking-request-form'
          onSubmit={form.handleSubmit(onSubmit)}
          className='flex min-w-0 flex-col gap-6'
          noValidate
        >
          <div className='grid gap-3'>
            <p className='type-overline text-fg-secondary'>Ruang &amp; waktu</p>
            <p className='type-card-title text-fg'>
              {room.code}
              <span className='type-body font-normal text-fg-secondary'>
                {' '}
                · {roomTypeConfig(room.type).label} · {room.capacity} orang
              </span>
            </p>
            <OccurrenceFields
              hours={hours}
              minDate={toWibDate(now)}
              maxDate={lastDate}
              maxDateHint={`Bisa dipilih sampai ${formatDateShort(`${lastDate}T00:00:00+07:00`)}`}
            />
          </div>

          <div className='grid gap-4'>
            <p className='type-overline text-fg-secondary'>Kegiatan</p>
            <div className='grid items-start gap-4 sm:grid-cols-[1fr_12rem]'>
              <FormField
                control={form.control}
                name='purpose'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Nama kegiatan</FormLabel>
                    <FormControl>
                      <Input
                        maxLength={PURPOSE_MAX}
                        placeholder='Contoh: Kuliah pengganti Kalkulus'
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='participants'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      Jumlah peserta
                      {participantsRequired(room.type) ? ' (wajib)' : ''}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        inputMode='numeric'
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
            <FormField
              control={form.control}
              name='notes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {room.type === 'AUDITORIUM'
                      ? 'Kebutuhan perlengkapan'
                      : 'Catatan (opsional)'}
                  </FormLabel>
                  <FormControl>
                    <Textarea
                      rows={3}
                      maxLength={NOTES_MAX}
                      placeholder='Tulis catatan singkat…'
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <LetterField required={needLetter} />
        </form>
      </Form>

      <RequestSummary
        room={room}
        occurrences={occurrences}
        participants={participants}
        outcome={outcome}
        conflicts={conflicts}
        problem={problem?.kind === 'other' ? problem.message : undefined}
        footnote={`${policy.approval_policy === 'AUTO_IF_ELIGIBLE' ? 'Ruang kosong disetujui otomatis bila riwayatmu baik. Selain itu menunggu validasi ' : 'Pengajuan menunggu validasi '}${approverLabel(policy)}.`}
        actions={
          <>
            <Button type='button' variant='secondary' onClick={onCancel}>
              Batal
            </Button>
            <Button
              type='submit'
              form='booking-request-form'
              variant='accent'
              disabled={cannotSubmit}
            >
              {create.isPending && <Loader2 className='animate-spin' />}
              Kirim Pengajuan
            </Button>
          </>
        }
      />
    </div>
  )
}
