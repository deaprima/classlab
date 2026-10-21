import { getRouteApi } from '@tanstack/react-router'
import { ChangePassword } from '.'

const route = getRouteApi('/change-password')

export function ChangePasswordPage() {
  const { redirect } = route.useSearch()
  return <ChangePassword redirectTo={redirect} />
}
