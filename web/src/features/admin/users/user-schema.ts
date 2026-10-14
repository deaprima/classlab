import { z } from 'zod'

// The API's account rules (decisions.md, Phase 1); the server re-checks them.
const ROLES = ['MAHASISWA', 'TU_KALAB', 'OB', 'KAPRODI'] as const

const name = z
  .string()
  .trim()
  .min(1, 'Nama wajib diisi.')
  .max(120, 'Nama maksimal 120 karakter.')
const email = z.string().trim().pipe(z.email('Format email tidak valid.'))
const password = z
  .string()
  .min(8, 'Kata sandi minimal 8 karakter.')
  .max(128, 'Kata sandi maksimal 128 karakter.')

export const createUserSchema = z.object({
  identifier: z
    .string()
    .trim()
    .regex(
      /^[A-Za-z0-9._-]{3,64}$/,
      'NIM/NIP 3-64 karakter (huruf, angka, titik, garis).'
    ),
  name,
  email,
  role: z.enum(ROLES, 'Pilih peran.'),
  password,
})
export type CreateUserValues = z.input<typeof createUserSchema>

export const editUserSchema = z.object({
  name,
  email,
  role: z.enum(ROLES),
  is_active: z.boolean(),
})
export type EditUserValues = z.input<typeof editUserSchema>

export const resetPasswordSchema = z
  .object({ password, confirm: z.string() })
  .refine((v) => v.password === v.confirm, {
    path: ['confirm'],
    message: 'Konfirmasi kata sandi tidak sama.',
  })
export type ResetPasswordValues = z.input<typeof resetPasswordSchema>
