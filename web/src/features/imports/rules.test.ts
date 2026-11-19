import { describe, expect, it } from 'vitest'
import {
  IMPORT_MAX_BYTES,
  checkCsvFile,
  credentialsCsv,
  csvCell,
  defaultSemester,
  rowErrorsFromDetails,
  semesterValue,
  summaryNumber,
} from './rules'

const file = (name: string, size = 10, type = 'text/csv') =>
  new File([new Uint8Array(size)], name, { type })

describe('checkCsvFile', () => {
  it('accepts a CSV and refuses other types, empty and oversized files', () => {
    expect(checkCsvFile(file('jadwal.csv'))).toBeNull()
    expect(checkCsvFile(file('JADWAL.CSV', 10, ''))).toBeNull()
    expect(checkCsvFile(file('jadwal.xlsx', 10, 'application/xlsx'))).toMatch(
      /CSV/
    )
    expect(checkCsvFile(file('kosong.csv', 0))).toMatch(/kosong/)
    expect(checkCsvFile(file('besar.csv', IMPORT_MAX_BYTES + 1))).toMatch(
      /2 MB/
    )
  })
})

describe('semester', () => {
  it('writes the form the API accepts', () => {
    expect(semesterValue(2026, 'GANJIL')).toBe('2026-GANJIL')
  })

  it('follows the academic calendar in WIB', () => {
    expect(defaultSemester(new Date('2026-10-22T05:00:00Z'))).toEqual({
      year: 2026,
      term: 'GANJIL',
    })
    expect(defaultSemester(new Date('2026-03-10T05:00:00Z'))).toEqual({
      year: 2026,
      term: 'GENAP',
    })
    // January still belongs to the odd semester that began the year before.
    expect(defaultSemester(new Date('2027-01-15T05:00:00Z'))).toEqual({
      year: 2026,
      term: 'GANJIL',
    })
  })
})

describe('credentials CSV', () => {
  it('quotes commas, quotes and line breaks', () => {
    expect(csvCell('plain')).toBe('plain')
    expect(csvCell('a,b')).toBe('"a,b"')
    expect(csvCell('say "hi"')).toBe('"say ""hi"""')
  })

  it('keeps a spreadsheet from running a cell as a formula', () => {
    expect(csvCell('=SUM(A1)')).toBe("'=SUM(A1)")
    expect(csvCell('+1')).toBe("'+1")
    expect(csvCell('-2')).toBe("'-2")
    expect(csvCell('@cmd')).toBe("'@cmd")
  })

  it('builds a header and one line per account, CRLF with a BOM', () => {
    const text = credentialsCsv([
      { identifier: '2023010090', temporary_password: 'k7Tn4pQ2xW' },
      { identifier: '2023010091', temporary_password: '-m3,Vb8z' },
    ])
    expect(text.charCodeAt(0)).toBe(0xfeff)
    expect(text.slice(1).split('\r\n')).toEqual([
      'identifier,temporary_password',
      '2023010090,k7Tn4pQ2xW',
      '2023010091,"\'-m3,Vb8z"',
      '',
    ])
  })
})

describe('server answers', () => {
  it('turns IMPORT_INVALID details into row errors', () => {
    expect(
      rowErrorsFromDetails([
        { line: 14, column: 'room_code', message: 'Ruang tidak dikenal' },
        { message: 'Berkas kosong' },
      ])
    ).toEqual([
      { line: 14, column: 'room_code', message: 'Ruang tidak dikenal' },
      { line: 0, message: 'Berkas kosong' },
    ])
  })

  it('reads a number from a summary, nothing else', () => {
    expect(summaryNumber({ created: 3, name: 'x' }, 'created')).toBe(3)
    expect(summaryNumber({ name: 'x' }, 'name')).toBeUndefined()
    expect(summaryNumber({}, 'created')).toBeUndefined()
  })
})
