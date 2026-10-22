import { type ImportRowError } from '@/lib/api-types'
import { toWibDate } from '@/lib/datetime'

// Mirrors IMPORT_MAX_MB of the API (default 2); the server decides, this only spares a slow upload.
export const IMPORT_MAX_BYTES = 2 * 1024 * 1024

/** An error text for a file that cannot be a CSV import, else null. */
export function checkCsvFile(file: File): string | null {
  const isCsv =
    /\.csv$/i.test(file.name) ||
    file.type === 'text/csv' ||
    file.type === 'application/vnd.ms-excel'
  if (!isCsv) return 'Berkas harus berformat CSV (.csv).'
  if (file.size === 0) return 'Berkas kosong.'
  if (file.size > IMPORT_MAX_BYTES) return 'Ukuran berkas maksimal 2 MB.'
  return null
}

export type Term = 'GANJIL' | 'GENAP'

/** "2026-GANJIL", the form the API accepts for `semester`. */
export function semesterValue(year: number, term: Term): string {
  return `${year}-${term}`
}

/** The running semester: odd from August, even from February; January belongs to the odd one before. */
export function defaultSemester(now: Date): { year: number; term: Term } {
  const [year, month] = toWibDate(now).split('-').map(Number)
  if (month >= 8) return { year, term: 'GANJIL' }
  if (month === 1) return { year: year - 1, term: 'GANJIL' }
  return { year, term: 'GENAP' }
}

/**
 * A CSV cell. Cells that start with = + - @ or a tab are prefixed with an apostrophe so a
 * spreadsheet never runs them as a formula; quotes, commas and line breaks are quoted.
 */
export function csvCell(value: string): string {
  const safe = /^[=+\-@\t\r]/.test(value) ? `'${value}` : value
  return /[",\r\n]/.test(safe) ? `"${safe.replace(/"/g, '""')}"` : safe
}

const BOM = String.fromCharCode(0xfeff)

/** The one-time credentials as a CSV text (CRLF, UTF-8 with BOM so Excel reads it right). */
export function credentialsCsv(
  rows: readonly { identifier: string; temporary_password: string }[]
): string {
  const lines = [
    'identifier,temporary_password',
    ...rows.map(
      (r) => `${csvCell(r.identifier)},${csvCell(r.temporary_password)}`
    ),
  ]
  return `${BOM}${lines.join('\r\n')}\r\n`
}

/** Saves a text as a file from the browser; nothing is sent anywhere. */
export function downloadText(name: string, text: string, type = 'text/csv') {
  const url = URL.createObjectURL(new Blob([text], { type }))
  const link = document.createElement('a')
  link.href = url
  link.download = name
  link.click()
  URL.revokeObjectURL(url)
}

/** Row errors of a 422 IMPORT_INVALID answer (`details` carry `line` and `column`). */
export function rowErrorsFromDetails(
  details: readonly { line?: number; column?: string; message: string }[]
): ImportRowError[] {
  return details.map((d) => ({
    line: d.line ?? 0,
    ...(d.column && { column: d.column }),
    message: d.message,
  }))
}

/** A number inside an import summary (its shape depends on the kind), or undefined. */
export function summaryNumber(
  summary: Record<string, unknown>,
  key: string
): number | undefined {
  const value = summary[key]
  return typeof value === 'number' ? value : undefined
}
