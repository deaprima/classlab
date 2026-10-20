import { z } from 'zod'

// The API's account rules (decisions.md, Phase 1); the server re-checks them.
const ROLES = ['MAHASISWA', 'ORGANISASI', 'TU', 'KALAB', 'OB', 'ADMIN'] as const

const name = z
  .string()
  .trim()
  .min(1, 'Nama wajib diisi.')
  .max(120, 'Nama maksimal 120 karakter.')
// Email is optional for every role; an empty field means "none".
const email = z
  .string()
  .trim()
  .refine(
    (v) => v === '' || z.email().safeParse(v).success,
    'Format email tidak valid.'
  )
// Server minimum is configurable (MIN_PASSWORD_LENGTH, default 10); the server re-checks it.
const password = z
  .string()
  .min(10, 'Kata sandi minimal 10 karakter.')
  .max(128, 'Kata sandi maksimal 128 karakter.')
const picName = z.string().trim().max(120, 'Nama PJ maksimal 120 karakter.')
const picPhone = z.string().trim().max(32, 'Telepon PJ maksimal 32 karakter.')

/** The person in charge (PIC) is required for ORGANISASI accounts and rejected for other roles. */
function requirePicForOrganisasi(
  v: { role: (typeof ROLES)[number]; pic_name: string; pic_phone: string },
  ctx: z.RefinementCtx
) {
  if (v.role !== 'ORGANISASI') return
  if (v.pic_name === '') {
    ctx.addIssue({
      code: 'custom',
      path: ['pic_name'],
      message: 'Nama penanggung jawab wajib diisi.',
    })
  }
  if (v.pic_phone === '') {
    ctx.addIssue({
      code: 'custom',
      path: ['pic_phone'],
      message: 'Telepon penanggung jawab wajib diisi.',
    })
  }
}

export const createUserSchema = z
  .object({
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
    pic_name: picName,
    pic_phone: picPhone,
  })
  .superRefine(requirePicForOrganisasi)
export type CreateUserValues = z.input<typeof createUserSchema>

export const editUserSchema = z
  .object({
    name,
    email,
    role: z.enum(ROLES),
    is_active: z.boolean(),
    pic_name: picName,
    pic_phone: picPhone,
  })
  .superRefine(requirePicForOrganisasi)
export type EditUserValues = z.input<typeof editUserSchema>
