import { useState } from 'react'
import {
  type Control,
  type FieldValues,
  type Path,
  useForm,
  useWatch,
} from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Check, Copy, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { formProblemFrom, getErrorMessage } from '@/lib/api-error'
import { type User } from '@/lib/api-types'
import { ROLE_LABELS } from '@/lib/rbac'
import { Button } from '@/components/ui/button'
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
import { PasswordInput } from '@/components/password-input'
import { useCreateUser, useResetPassword, useUpdateUser } from './api'
import {
  type CreateUserValues,
  type EditUserValues,
  createUserSchema,
  editUserSchema,
} from './user-schema'

export type UserDialogState =
  | { mode: 'create' }
  | { mode: 'edit'; user: User }
  | { mode: 'password'; user: User }
  | null

// API field -> form field, for 422 details (e.g. duplicate identifier or email).
const CREATE_FIELDS = {
  identifier: 'identifier',
  name: 'name',
  email: 'email',
  role: 'role',
  password: 'password',
  pic_name: 'pic_name',
  pic_phone: 'pic_phone',
} as const
const EDIT_FIELDS = {
  name: 'name',
  email: 'email',
  role: 'role',
  is_active: 'is_active',
  pic_name: 'pic_name',
  pic_phone: 'pic_phone',
} as const

/** Content mounts only while open, so each dialog starts from fresh values. */
export function UserDialog({
  state,
  onClose,
}: {
  state: UserDialogState
  onClose: () => void
}) {
  return (
    <Dialog open={state !== null} onOpenChange={(o) => !o && onClose()}>
      {state?.mode === 'create' && <CreateUserContent onDone={onClose} />}
      {state?.mode === 'edit' && (
        <EditUserContent user={state.user} onDone={onClose} />
      )}
      {state?.mode === 'password' && (
        <ResetPasswordContent user={state.user} onDone={onClose} />
      )}
    </Dialog>
  )
}

function CreateUserContent({ onDone }: { onDone: () => void }) {
  const orgUnitId = useAuthStore((s) => s.auth.user?.org_unit_id)
  const [problem, setProblem] = useState<FormProblem | null>(null)
  const form = useForm<CreateUserValues>({
    resolver: zodResolver(createUserSchema),
    defaultValues: {
      identifier: '',
      name: '',
      email: '',
      role: 'MAHASISWA',
      password: '',
      pic_name: '',
      pic_phone: '',
    },
  })
  const role = useWatch({ control: form.control, name: 'role' })
  const create = useCreateUser({
    onSuccess: (u) => {
      toast.success(`Akun ${u.name} (${u.identifier}) dibuat.`)
      onDone()
    },
    onError: (e) =>
      setProblem(formProblemFrom(e, form.setError, CREATE_FIELDS)),
  })

  const submit = form.handleSubmit((v) => {
    setProblem(null)
    if (!orgUnitId) return
    // MVP has one unit; new accounts belong to the admin's own (decisions.md, Phase 5).
    create.mutate({
      org_unit_id: orgUnitId,
      identifier: v.identifier.trim(),
      name: v.name.trim(),
      // Email is optional; the PIC fields exist for ORGANISASI only.
      ...(v.email.trim() && { email: v.email.trim() }),
      role: v.role,
      password: v.password,
      ...(v.role === 'ORGANISASI' && {
        pic_name: v.pic_name.trim(),
        pic_phone: v.pic_phone.trim(),
      }),
    })
  })

  return (
    <DialogContent className='sm:max-w-lg'>
      <DialogHeader>
        <DialogTitle>Tambah pengguna</DialogTitle>
        <DialogDescription>
          Tidak ada registrasi mandiri: berikan kata sandi sementara ini kepada
          pengguna secara langsung. Pengguna wajib menggantinya saat pertama
          masuk.
        </DialogDescription>
      </DialogHeader>
      <Form {...form}>
        <form
          id='user-form'
          noValidate
          className='grid gap-4'
          onSubmit={submit}
        >
          <FormProblemAlert problem={problem} />
          <div className='grid items-start gap-4 sm:grid-cols-2'>
            <TextField
              control={form.control}
              name='identifier'
              label='NIM / NIP'
            />
            <RoleField control={form.control} name='role' />
          </div>
          <TextField control={form.control} name='name' label='Nama' />
          <TextField
            control={form.control}
            name='email'
            label='Email (opsional)'
            type='email'
          />
          {role === 'ORGANISASI' && (
            <div className='grid items-start gap-4 sm:grid-cols-2'>
              <TextField
                control={form.control}
                name='pic_name'
                label='Nama penanggung jawab'
              />
              <TextField
                control={form.control}
                name='pic_phone'
                label='Telepon penanggung jawab'
                type='tel'
              />
            </div>
          )}
          <FormField
            control={form.control}
            name='password'
            render={({ field }) => (
              <FormItem>
                <FormLabel>Kata sandi sementara</FormLabel>
                <FormControl>
                  <PasswordInput autoComplete='new-password' {...field} />
                </FormControl>
                <FormDescription>Minimal 10 karakter.</FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </form>
      </Form>
      <Footer
        pending={create.isPending}
        onCancel={onDone}
        formId='user-form'
        label='Buat akun'
      />
    </DialogContent>
  )
}

function EditUserContent({ user, onDone }: { user: User; onDone: () => void }) {
  const isSelf = useAuthStore((s) => s.auth.user?.id === user.id)
  const [problem, setProblem] = useState<FormProblem | null>(null)
  const form = useForm<EditUserValues>({
    resolver: zodResolver(editUserSchema),
    defaultValues: {
      name: user.name,
      email: user.email ?? '',
      role: user.role,
      is_active: user.is_active,
      pic_name: user.pic_name ?? '',
      pic_phone: user.pic_phone ?? '',
    },
  })
  const role = useWatch({ control: form.control, name: 'role' })
  const update = useUpdateUser({
    onSuccess: (u) => {
      toast.success(`Akun ${u.name} diperbarui.`)
      onDone()
    },
    onError: (e) => setProblem(formProblemFrom(e, form.setError, EDIT_FIELDS)),
  })

  const submit = form.handleSubmit((v) => {
    setProblem(null)
    update.mutate({
      id: user.id,
      // Own role and status are not sent: the API refuses to change them (decisions.md, Phase 1).
      body: {
        name: v.name.trim(),
        // An empty email is left out: the API cannot clear an address.
        ...(v.email.trim() && { email: v.email.trim() }),
        ...(!isSelf && { role: v.role, is_active: v.is_active }),
        ...(v.role === 'ORGANISASI' && {
          pic_name: v.pic_name.trim(),
          pic_phone: v.pic_phone.trim(),
        }),
      },
    })
  })

  return (
    <DialogContent className='sm:max-w-lg'>
      <DialogHeader>
        <DialogTitle>Ubah {user.name}</DialogTitle>
        <DialogDescription>
          NIM/NIP {user.identifier} tidak dapat diubah.
        </DialogDescription>
      </DialogHeader>
      <Form {...form}>
        <form
          id='user-form'
          noValidate
          className='grid gap-4'
          onSubmit={submit}
        >
          <FormProblemAlert problem={problem} />
          <TextField control={form.control} name='name' label='Nama' />
          <TextField
            control={form.control}
            name='email'
            label='Email (opsional)'
            type='email'
          />
          <RoleField control={form.control} name='role' disabled={isSelf} />
          {role === 'ORGANISASI' && (
            <div className='grid items-start gap-4 sm:grid-cols-2'>
              <TextField
                control={form.control}
                name='pic_name'
                label='Nama penanggung jawab'
              />
              <TextField
                control={form.control}
                name='pic_phone'
                label='Telepon penanggung jawab'
                type='tel'
              />
            </div>
          )}
          <FormField
            control={form.control}
            name='is_active'
            render={({ field }) => (
              <FormItem className='flex items-center justify-between gap-4 rounded-md border p-3'>
                <div className='space-y-0.5'>
                  <FormLabel>Akun aktif</FormLabel>
                  <FormDescription>
                    {isSelf
                      ? 'Anda tidak dapat menonaktifkan atau mengubah peran akun sendiri.'
                      : 'Menonaktifkan akun langsung mengakhiri semua sesinya.'}
                  </FormDescription>
                </div>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={isSelf}
                  />
                </FormControl>
              </FormItem>
            )}
          />
        </form>
      </Form>
      <Footer
        pending={update.isPending}
        onCancel={onDone}
        formId='user-form'
        label='Simpan'
      />
    </DialogContent>
  )
}

/**
 * Two steps in one dialog: confirm, then show the temporary password. The password lives only in
 * the mutation state, so closing the dialog discards it for good.
 */
function ResetPasswordContent({
  user,
  onDone,
}: {
  user: User
  onDone: () => void
}) {
  const [problem, setProblem] = useState<FormProblem | null>(null)
  const reset = useResetPassword({
    onError: (e) => setProblem({ message: getErrorMessage(e) }),
  })

  if (reset.data) {
    return (
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>Kata sandi sementara</DialogTitle>
          <DialogDescription>
            Berikan kata sandi ini kepada {user.name} ({user.identifier}). Kata
            sandi hanya ditampilkan sekali dan tidak dapat dilihat lagi setelah
            dialog ini ditutup. Pengguna wajib menggantinya saat masuk.
          </DialogDescription>
        </DialogHeader>
        <TemporaryPassword value={reset.data.temporary_password} />
        <DialogFooter>
          <Button onClick={onDone}>Selesai</Button>
        </DialogFooter>
      </DialogContent>
    )
  }

  return (
    <DialogContent className='sm:max-w-md'>
      <DialogHeader>
        <DialogTitle>Reset kata sandi</DialogTitle>
        <DialogDescription>
          {user.name} ({user.identifier}) akan keluar dari semua perangkat dan
          menerima kata sandi sementara acak yang harus diganti saat masuk.
        </DialogDescription>
      </DialogHeader>
      <FormProblemAlert problem={problem} />
      <DialogFooter>
        <Button variant='outline' disabled={reset.isPending} onClick={onDone}>
          Batal
        </Button>
        <Button
          disabled={reset.isPending}
          onClick={() => {
            setProblem(null)
            reset.mutate(user.id)
          }}
        >
          {reset.isPending && <Loader2 className='animate-spin' />}
          Reset kata sandi
        </Button>
      </DialogFooter>
    </DialogContent>
  )
}

function TemporaryPassword({ value }: { value: string }) {
  const [copied, setCopied] = useState(false)
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
    } catch {
      toast.error('Tidak dapat menyalin. Salin kata sandi secara manual.')
    }
  }
  return (
    <div className='flex items-center gap-2'>
      <code
        aria-label='Kata sandi sementara'
        className='min-w-0 flex-1 rounded-md border bg-muted px-3 py-2 font-mono text-base break-all select-all'
      >
        {value}
      </code>
      <Button
        variant='outline'
        size='icon'
        aria-label='Salin kata sandi'
        onClick={copy}
      >
        {copied ? <Check /> : <Copy />}
      </Button>
    </div>
  )
}

function TextField<T extends FieldValues>({
  control,
  name,
  label,
  type = 'text',
}: {
  control: Control<T>
  name: Path<T>
  label: string
  type?: string
}) {
  return (
    <FormField
      control={control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{label}</FormLabel>
          <FormControl>
            <Input type={type} {...field} />
          </FormControl>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}

function RoleField<T extends FieldValues>({
  control,
  name,
  disabled,
}: {
  control: Control<T>
  name: Path<T>
  disabled?: boolean
}) {
  return (
    <FormField
      control={control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>Peran</FormLabel>
          <Select
            value={field.value}
            onValueChange={field.onChange}
            disabled={disabled}
          >
            <FormControl>
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
            </FormControl>
            <SelectContent>
              {Object.entries(ROLE_LABELS).map(([value, label]) => (
                <SelectItem key={value} value={value}>
                  {label}
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

function Footer({
  pending,
  onCancel,
  formId,
  label,
}: {
  pending: boolean
  onCancel: () => void
  formId: string
  label: string
}) {
  return (
    <DialogFooter>
      <Button variant='outline' disabled={pending} onClick={onCancel}>
        Batal
      </Button>
      <Button type='submit' form={formId} disabled={pending}>
        {pending && <Loader2 className='animate-spin' />}
        {label}
      </Button>
    </DialogFooter>
  )
}
