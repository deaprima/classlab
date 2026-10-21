import { useEffect, useMemo, useRef, useState } from 'react'
import { useForm, useWatch } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Link } from '@tanstack/react-router'
import { attendanceRules } from '@/config/attendance-rules'
import { AlertCircle, Loader2 } from 'lucide-react'
import { useAuthStore } from '@/stores/auth-store'
import { getApiError, getErrorMessage, formProblemFrom } from '@/lib/api-error'
import {
  type Booking,
  type BookingGroupResult,
  type Room,
  type RoomType,
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'
import { type ConflictItem } from '@/components/conflict-list'
import { SegmentedControl } from '@/components/segmented-control'
import { useMe } from '@/features/auth/api'
import { useRoomTypes } from '@/features/room-types/api'
import { roomTypeConfig } from '@/features/room-types/config'
import { useRooms } from '@/features/rooms/api'
import { useCreateBooking, useIdempotencyKey } from '../api'
import { LetterField } from './letter-field'
import { OccurrenceFields } from './occurrence-fields'
import {
  approverLabel,
  availableGroups,
  effectiveHours,
  groupOf,
  letterRequired,
  participantsRequired,
  previewOutcome,
  requestablePolicies,
  TYPE_GROUP_LABELS,
  type TypeGroup,
} from './policy'
import {
  createRequestSchema,
  EMPTY_REQUEST,
  lastBookableDate,
  NOTES_MAX,
  PURPOSE_MAX,
  REQUEST_FIELD_MAP,
  toCreateBookingRequest,
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
  /** Fixes the room type (e.g. "Ajukan Auditorium"): no type switch, only rooms of that type. */
  lockedType?: RoomType
  initial?: { room_id?: string; date?: string; start?: string; end?: string }
  onCreated: (result: Booking | BookingGroupResult) => void
}

const FORM_ID = 'booking-request-form'

export function RequestForm({
  lockedType,
  initial,
  onCreated,
}: RequestFormProps) {
  const user = useAuthStore((state) => state.auth.user)
  const me = useMe()
  const policiesQuery = useRoomTypes()
  const roomsQuery = useRooms({ is_active: true, per_page: 100 })
  const keyFor = useIdempotencyKey()
  const [problem, setProblem] = useState<Problem | null>(null)
  const role = user?.role ?? 'MAHASISWA'

  const policies = useMemo(
    () =>
      requestablePolicies(policiesQuery.data ?? [], role).filter(
        (p) => !lockedType || p.type === lockedType
      ),
    [policiesQuery.data, role, lockedType]
  )
  const rooms = useMemo(() => {
    const types = new Set(policies.map((p) => p.type))
    return (roomsQuery.data?.data ?? []).filter((r) => types.has(r.type))
  }, [roomsQuery.data, policies])
  const groups = availableGroups(policies)

  const initialRoom = rooms.find((r) => r.id === initial?.room_id)
  const [chosenGroup, setGroup] = useState<TypeGroup | undefined>()
  const group: TypeGroup | undefined =
    chosenGroup && groups.includes(chosenGroup)
      ? chosenGroup
      : initialRoom
        ? groupOf(initialRoom.type)
        : groups[0]

  // The schema depends on the room picked, which lives in the form: the resolver reads it lazily.
  const contextRef = useRef<RequestContext>({ role })
  const form = useForm<RequestFormValues>({
    resolver: (values, context, options) =>
      zodResolver(createRequestSchema(contextRef.current))(
        values,
        context,
        options
      ),
    defaultValues: {
      ...EMPTY_REQUEST,
      room_id: initial?.room_id ?? '',
      occurrences: [
        {
          date: initial?.date ?? '',
          start_time: initial?.start ?? '',
          end_time: initial?.end ?? '',
        },
      ],
    },
  })

  const roomId = useWatch({ control: form.control, name: 'room_id' })
  const occurrences = useWatch({ control: form.control, name: 'occurrences' })
  const participants = useWatch({ control: form.control, name: 'participants' })
  const room: Room | undefined = rooms.find((r) => r.id === roomId)
  const policy = policies.find((p) => p.type === room?.type)
  // Read by the resolver on submit and on change, which happen after this effect has run.
  useEffect(() => {
    contextRef.current = { role, room, policy }
  })

  // A group with a single room (the auditorium) has nothing to choose: select it.
  const only = rooms.filter((r) => !group || groupOf(r.type) === group)
  const soleRoomId = only.length === 1 ? only[0].id : undefined
  useEffect(() => {
    if (soleRoomId && !form.getValues('room_id')) {
      form.setValue('room_id', soleRoomId)
    }
  }, [soleRoomId, form])

  const slot = useSlotCheck(roomId, occurrences)
  const blockedUntil =
    problem?.kind === 'blocked'
      ? problem.until
      : (me.data?.booking_blocked_until ?? undefined)
  const blocked = !!blockedUntil && new Date(blockedUntil) > new Date()

  const outcome = previewOutcome({
    policy: policy ?? policies[0],
    role,
    blocked,
    slotFree: policy ? slot.free : null,
    approverLabel: policy ? approverLabel(policy) : '',
  })
  const conflictsToShow: ConflictItem[] =
    problem?.kind === 'conflict' ? problem.items : slot.conflicts

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

  function pickGroup(next: TypeGroup) {
    setGroup(next)
    form.setValue('room_id', '')
    setProblem(null)
  }

  const now = new Date()
  const today = toWibDate(now)
  const hours =
    room && policy
      ? effectiveHours(room, policy)
      : { open: '07:00', close: '21:00' }
  const lastDate = policy ? lastBookableDate(policy, now) : undefined
  const needLetter = policy
    ? letterRequired(policy, role)
    : role === 'ORGANISASI'
  const roomsInGroup = rooms.filter((r) => !group || groupOf(r.type) === group)
  const cannotSubmit =
    create.isPending ||
    blocked ||
    outcome.kind === 'conflict' ||
    problem?.kind === 'conflict'

  if (policiesQuery.isPending || roomsQuery.isPending) {
    return <Skeleton className='h-96 rounded-2xl' />
  }
  if (policiesQuery.isError || roomsQuery.isError) {
    return (
      <Alert variant='error'>
        <AlertCircle />
        <AlertDescription>
          {getErrorMessage(policiesQuery.error ?? roomsQuery.error)}
        </AlertDescription>
      </Alert>
    )
  }
  if (policies.length === 0) {
    return (
      <Alert variant='info'>
        <AlertCircle />
        <AlertTitle>Tidak ada ruang yang bisa diajukan</AlertTitle>
        <AlertDescription>
          Perannya tidak termasuk peminta untuk tipe ruang mana pun.
        </AlertDescription>
      </Alert>
    )
  }

  const submitLabel = 'Kirim Pengajuan'
  const actions = (
    <>
      <Button
        type='submit'
        form={FORM_ID}
        variant='accent'
        disabled={cannotSubmit}
      >
        {create.isPending && <Loader2 className='animate-spin' />}
        {submitLabel}
      </Button>
      <Button type='button' variant='secondary' asChild>
        <Link to='/'>Batal</Link>
      </Button>
    </>
  )

  return (
    <>
      {blocked && blockedUntil && (
        <Alert variant='error' className='mb-5'>
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
        <Alert variant='error' className='mb-5'>
          <AlertCircle />
          <AlertTitle>Kuota booking aktif penuh</AlertTitle>
          <AlertDescription>{problem.message}</AlertDescription>
        </Alert>
      )}

      <div className='grid grid-cols-1 gap-5 lg:grid-cols-[minmax(0,1fr)_340px]'>
        <Form {...form}>
          <form
            id={FORM_ID}
            onSubmit={form.handleSubmit(onSubmit)}
            className='flex min-w-0 flex-col gap-6 rounded-2xl border bg-card p-5'
            noValidate
          >
            <h2 className='type-section text-fg'>Detail Pengajuan</h2>

            <div className='grid gap-4'>
              <p className='type-overline text-fg-secondary'>
                Ruang &amp; waktu
              </p>
              {!lockedType && groups.length > 1 && group && (
                <SegmentedControl
                  aria-label='Jenis ruang'
                  value={group}
                  onValueChange={pickGroup}
                  options={groups.map((g) => ({
                    value: g,
                    label: TYPE_GROUP_LABELS[g],
                  }))}
                />
              )}

              <FormField
                control={form.control}
                name='room_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Ruang</FormLabel>
                    <Select value={field.value} onValueChange={field.onChange}>
                      <FormControl>
                        <SelectTrigger className='w-full'>
                          <SelectValue placeholder='Pilih ruang' />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        {roomsInGroup.map((r) => (
                          <SelectItem key={r.id} value={r.id}>
                            {r.code} · {roomTypeConfig(r.type).label} ·{' '}
                            {r.capacity} orang
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <OccurrenceFields
                hours={hours}
                minDate={today}
                maxDate={lastDate}
                maxDateHint={
                  lastDate
                    ? `Bisa dipilih sampai ${formatDateShort(`${lastDate}T00:00:00+07:00`)}`
                    : undefined
                }
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
                        {room && participantsRequired(room.type)
                          ? ' (wajib)'
                          : ''}
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
                      {room?.type === 'AUDITORIUM'
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

            {(needLetter || policy) && <LetterField required={needLetter} />}
          </form>
        </Form>

        <div className='flex min-w-0 flex-col gap-3'>
          <RequestSummary
            room={room}
            occurrences={occurrences}
            participants={participants}
            outcome={
              problem?.kind === 'conflict'
                ? {
                    kind: 'conflict',
                    title: problem.message,
                    description: 'Pilih jam lain, lalu kirim ulang.',
                  }
                : outcome
            }
            conflicts={conflictsToShow}
            problem={problem?.kind === 'other' ? problem.message : undefined}
            actions={actions}
            footnote={
              policy
                ? `${policy.approval_policy === 'AUTO_IF_ELIGIBLE' ? 'Ruang kosong disetujui otomatis bila riwayatmu baik. Selain itu menunggu validasi ' : 'Pengajuan menunggu validasi '}${approverLabel(policy)}.`
                : undefined
            }
          />
        </div>
      </div>

      <div className='h-20 md:hidden' aria-hidden />
    </>
  )
}
