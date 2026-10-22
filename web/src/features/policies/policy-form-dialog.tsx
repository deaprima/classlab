import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2 } from 'lucide-react'
import { formProblemFrom } from '@/lib/api-error'
import { type RoomTypePolicy, type UserRole } from '@/lib/api-types'
import { formatDuration } from '@/lib/datetime'
import { ROLE_LABELS } from '@/lib/rbac'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Form,
  FormControl,
  FormDescription,
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
import { Switch } from '@/components/ui/switch'
import { type FormProblem, FormProblemAlert } from '@/components/form-problem'
import { useUpdatePolicy } from './api'
import {
  APPROVER_ROLES,
  CLOSE_HOURS,
  DURATIONS,
  HOURS,
  POLICY_FIELDS,
  type PolicyFormValues,
  REQUESTER_ROLES,
  policyFormSchema,
} from './schema'

/** Edit the policy of one room type. Content mounts only while open, so it starts from the saved values. */
export function PolicyFormDialog({
  policy,
  onClose,
}: {
  policy: RoomTypePolicy | null
  onClose: () => void
}) {
  return (
    <Dialog open={policy !== null} onOpenChange={(o) => !o && onClose()}>
      {policy && <PolicyFormContent policy={policy} onDone={onClose} />}
    </Dialog>
  )
}

function PolicyFormContent({
  policy,
  onDone,
}: {
  policy: RoomTypePolicy
  onDone: () => void
}) {
  const [problem, setProblem] = useState<FormProblem | null>(null)
  const form = useForm<PolicyFormValues>({
    resolver: zodResolver(policyFormSchema),
    defaultValues: {
      approver_roles:
        policy.approver_roles as PolicyFormValues['approver_roles'],
      requester_roles:
        policy.requester_roles as PolicyFormValues['requester_roles'],
      approval_policy: policy.approval_policy,
      letter_required: policy.letter_required,
      open_time: policy.open_time,
      close_time: policy.close_time,
      min_duration_min: policy.min_duration_min,
      max_duration_min: policy.max_duration_min,
      max_advance_days: String(policy.max_advance_days),
    },
  })
  const update = useUpdatePolicy({
    onSuccess: onDone,
    onError: (e) =>
      setProblem(formProblemFrom(e, form.setError, POLICY_FIELDS)),
  })

  const submit = form.handleSubmit((v) => {
    setProblem(null)
    update.mutate({
      type: policy.type,
      body: {
        approver_roles: v.approver_roles,
        requester_roles: v.requester_roles,
        approval_policy: v.approval_policy,
        letter_required: v.letter_required,
        open_time: v.open_time,
        close_time: v.close_time,
        min_duration_min: v.min_duration_min,
        max_duration_min: v.max_duration_min,
        max_advance_days: Number(v.max_advance_days),
      },
    })
  })

  return (
    <DialogContent className='max-h-[90dvh] overflow-y-auto sm:max-w-lg'>
      <DialogHeader>
        <DialogTitle>Edit Kebijakan · {policy.label}</DialogTitle>
        <DialogDescription>
          Berlaku untuk pengajuan berikutnya; booking yang sudah diputuskan
          tidak berubah. Perubahan tercatat di audit log.
        </DialogDescription>
      </DialogHeader>
      <Form {...form}>
        <form
          id='policy-form'
          noValidate
          className='grid gap-4'
          onSubmit={submit}
        >
          <FormProblemAlert problem={problem} />
          <RoleChecks
            form={form}
            name='approver_roles'
            label='Approver'
            roles={APPROVER_ROLES}
          />
          <RoleChecks
            form={form}
            name='requester_roles'
            label='Boleh diajukan oleh'
            roles={REQUESTER_ROLES}
          />
          <FormField
            control={form.control}
            name='approval_policy'
            render={({ field }) => (
              <FormItem>
                <FormLabel>Persetujuan</FormLabel>
                <Select value={field.value} onValueChange={field.onChange}>
                  <FormControl>
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    <SelectItem value='AUTO_IF_ELIGIBLE'>
                      Otomatis bila memenuhi syarat
                    </SelectItem>
                    <SelectItem value='ALWAYS_MANUAL'>Selalu manual</SelectItem>
                  </SelectContent>
                </Select>
                <FormDescription>
                  Otomatis = disetujui sistem bila ruang kosong dan riwayat
                  pemohon baik.
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
          <div className='grid gap-4 sm:grid-cols-2'>
            <SelectField
              form={form}
              name='open_time'
              label='Buka'
              options={HOURS.map((h) => ({ value: h, label: h }))}
            />
            <SelectField
              form={form}
              name='close_time'
              label='Tutup'
              options={CLOSE_HOURS.map((h) => ({ value: h, label: h }))}
            />
          </div>
          <div className='grid gap-4 sm:grid-cols-2'>
            <NumberSelect
              form={form}
              name='min_duration_min'
              label='Durasi min'
            />
            <NumberSelect
              form={form}
              name='max_duration_min'
              label='Durasi maks'
            />
          </div>
          <FormField
            control={form.control}
            name='max_advance_days'
            render={({ field }) => (
              <FormItem>
                <FormLabel>Maks hari ke depan</FormLabel>
                <FormControl>
                  <Input type='number' min={1} inputMode='numeric' {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='letter_required'
            render={({ field }) => (
              <FormItem className='flex items-center justify-between gap-4 rounded-md border p-3'>
                <FormLabel>Surat wajib</FormLabel>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </FormItem>
            )}
          />
        </form>
      </Form>
      <DialogFooter>
        <Button variant='outline' disabled={update.isPending} onClick={onDone}>
          Batal
        </Button>
        <Button type='submit' form='policy-form' disabled={update.isPending}>
          {update.isPending && <Loader2 className='animate-spin' />}
          Simpan
        </Button>
      </DialogFooter>
    </DialogContent>
  )
}

function RoleChecks<N extends 'approver_roles' | 'requester_roles'>({
  form,
  name,
  label,
  roles,
}: {
  form: ReturnType<typeof useForm<PolicyFormValues>>
  name: N
  label: string
  roles: readonly UserRole[]
}) {
  return (
    <FormField
      control={form.control}
      name={name}
      render={({ field }) => {
        const chosen = field.value as string[]
        return (
          <FormItem>
            <FormLabel>{label}</FormLabel>
            <div className='flex flex-wrap gap-4'>
              {roles.map((role) => (
                <label key={role} className='flex items-center gap-2 type-body'>
                  <Checkbox
                    checked={chosen.includes(role)}
                    onCheckedChange={(on) =>
                      field.onChange(
                        on
                          ? [...chosen, role]
                          : chosen.filter((r) => r !== role)
                      )
                    }
                  />
                  {ROLE_LABELS[role]}
                </label>
              ))}
            </div>
            <FormMessage />
          </FormItem>
        )
      }}
    />
  )
}

function SelectField({
  form,
  name,
  label,
  options,
}: {
  form: ReturnType<typeof useForm<PolicyFormValues>>
  name: 'open_time' | 'close_time'
  label: string
  options: { value: string; label: string }[]
}) {
  return (
    <FormField
      control={form.control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{label}</FormLabel>
          <Select value={field.value} onValueChange={field.onChange}>
            <FormControl>
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
            </FormControl>
            <SelectContent>
              {options.map((o) => (
                <SelectItem key={o.value} value={o.value}>
                  {o.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}

function NumberSelect({
  form,
  name,
  label,
}: {
  form: ReturnType<typeof useForm<PolicyFormValues>>
  name: 'min_duration_min' | 'max_duration_min'
  label: string
}) {
  return (
    <FormField
      control={form.control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{label}</FormLabel>
          <Select
            value={String(field.value)}
            onValueChange={(v) => field.onChange(Number(v))}
          >
            <FormControl>
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
            </FormControl>
            <SelectContent>
              {DURATIONS.map((m) => (
                <SelectItem key={m} value={String(m)}>
                  {formatDuration(m)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}
