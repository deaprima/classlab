import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { StudentImportPage } from '@/features/imports'

export const Route = createFileRoute('/_authenticated/manage/students/import')({
  beforeLoad: () => requireRole(ACCESS.manageImports),
  component: StudentImportPage,
})
