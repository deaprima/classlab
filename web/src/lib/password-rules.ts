import { z } from 'zod'

// Mirrors the server default (MIN_PASSWORD_LENGTH); the server stays the authority and answers
// 422 on `new_password` when its configured minimum is higher.
export const MIN_PASSWORD_LENGTH = 10
const MAX_PASSWORD_LENGTH = 128

export const changePasswordSchema = z
  .object({
    current_password: z.string().min(1, 'Kata sandi saat ini wajib diisi.'),
    new_password: z
      .string()
      .min(
        MIN_PASSWORD_LENGTH,
        `Kata sandi baru minimal ${MIN_PASSWORD_LENGTH} karakter.`
      )
      .max(
        MAX_PASSWORD_LENGTH,
        `Kata sandi baru maksimal ${MAX_PASSWORD_LENGTH} karakter.`
      ),
    confirm_password: z.string().min(1, 'Ulangi kata sandi baru.'),
  })
  .refine((v) => v.new_password === v.confirm_password, {
    path: ['confirm_password'],
    message: 'Kata sandi tidak sama.',
  })
  .refine((v) => v.new_password !== v.current_password, {
    path: ['new_password'],
    message: 'Kata sandi baru harus berbeda dari yang sekarang.',
  })

export type ChangePasswordValues = z.infer<typeof changePasswordSchema>
