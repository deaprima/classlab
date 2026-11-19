import { describe, expect, it } from 'vitest'
import { changePasswordSchema, MIN_PASSWORD_LENGTH } from './password-rules'

const valid = {
  current_password: 'sementara-123',
  new_password: 'kata-sandi-baru',
  confirm_password: 'kata-sandi-baru',
}

function firstMessage(input: unknown, path: string) {
  const result = changePasswordSchema.safeParse(input)
  if (result.success) return undefined
  return result.error.issues.find((i) => i.path.join('.') === path)?.message
}

describe('changePasswordSchema', () => {
  it('accepts a valid change', () => {
    expect(changePasswordSchema.safeParse(valid).success).toBe(true)
  })

  it('requires the current password', () => {
    expect(
      firstMessage({ ...valid, current_password: '' }, 'current_password')
    ).toMatch(/wajib/)
  })

  it(`rejects a new password shorter than ${MIN_PASSWORD_LENGTH}`, () => {
    const short = 'a'.repeat(MIN_PASSWORD_LENGTH - 1)
    expect(
      firstMessage(
        { ...valid, new_password: short, confirm_password: short },
        'new_password'
      )
    ).toMatch(/minimal 10/)
    const exact = 'a'.repeat(MIN_PASSWORD_LENGTH)
    expect(
      changePasswordSchema.safeParse({
        ...valid,
        new_password: exact,
        confirm_password: exact,
      }).success
    ).toBe(true)
  })

  it('rejects a new password over 128 characters', () => {
    const long = 'a'.repeat(129)
    expect(
      firstMessage(
        { ...valid, new_password: long, confirm_password: long },
        'new_password'
      )
    ).toMatch(/maksimal 128/)
  })

  it('rejects a confirmation that differs', () => {
    expect(
      firstMessage(
        { ...valid, confirm_password: 'lain-lagi-ya' },
        'confirm_password'
      )
    ).toBe('Kata sandi tidak sama.')
  })

  it('rejects reusing the current password', () => {
    expect(
      firstMessage(
        {
          current_password: 'sama-sama-123',
          new_password: 'sama-sama-123',
          confirm_password: 'sama-sama-123',
        },
        'new_password'
      )
    ).toMatch(/berbeda/)
  })
})
