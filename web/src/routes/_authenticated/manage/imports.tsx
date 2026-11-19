import { createFileRoute } from '@tanstack/react-router'
import { ACCESS, requireRole } from '@/lib/rbac'
import { ScheduleImportPage } from '@/features/imports'

// Class schedule import; the student accounts have their own page under /manage/students/import.
export const Route = createFileRoute('/_authenticated/manage/imports')({
  beforeLoad: () => requireRole(ACCESS.manageImports),
  component: ScheduleImportPage,
})
