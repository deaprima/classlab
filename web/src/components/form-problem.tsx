import { AlertCircle } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { type ConflictItem, ConflictList } from '@/components/conflict-list'

/** A form-level error: a message, optionally with the bookings/schedules that clash (409). */
export type FormProblem = {
  message: string
  conflicts?: ConflictItem[]
}

export function FormProblemAlert({ problem }: { problem: FormProblem | null }) {
  if (!problem) return null
  return (
    <Alert variant='destructive' role='alert'>
      <AlertCircle />
      {problem.conflicts && problem.conflicts.length > 0 && (
        <AlertTitle>Jadwal bentrok</AlertTitle>
      )}
      <AlertDescription>
        <p>{problem.message}</p>
        {problem.conflicts && (
          <ConflictList items={problem.conflicts} withDate />
        )}
      </AlertDescription>
    </Alert>
  )
}
