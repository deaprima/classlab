import { sampleUser } from '@/test-utils/mock-adapter'
import { type Booking, type Room, type RoomTypePolicy } from '@/lib/api-types'
import { addDays, todayWib } from '@/lib/datetime'

/** Test data shared by the "Pengajuan Saya" tests. */

const LAB: Room = {
  id: 'room-lab',
  org_unit_id: sampleUser.org_unit_id,
  code: 'LAB-RPL-02',
  name: 'Lab RPL',
  type: 'LAB',
  capacity: 32,
  facilities: [],
  is_active: true,
  open_time: null,
  close_time: null,
}

export const KELAS: Room = {
  ...LAB,
  id: 'room-kelas',
  code: 'R-401',
  name: 'R-401',
  type: 'KELAS',
  capacity: 60,
}

export const POLICIES: RoomTypePolicy[] = [
  {
    type: 'LAB',
    label: 'Laboratorium',
    approver_roles: ['KALAB'],
    requester_roles: ['MAHASISWA'],
    approval_policy: 'ALWAYS_MANUAL',
    letter_required: false,
    open_time: '07:00',
    close_time: '21:00',
    min_duration_min: 30,
    max_duration_min: 480,
    max_advance_days: 30,
    updated_at: 'x',
  },
  {
    type: 'KELAS',
    label: 'Ruang Kelas',
    approver_roles: ['TU'],
    requester_roles: ['MAHASISWA'],
    approval_policy: 'AUTO_IF_ELIGIBLE',
    letter_required: false,
    open_time: '07:00',
    close_time: '21:00',
    min_duration_min: 30,
    max_duration_min: 480,
    max_advance_days: 30,
    updated_at: 'x',
  },
]

/** A date `days` from today (WIB) at the given hours, as API timestamps. */
export function slotOn(days: number, startHour: number, endHour: number) {
  const day = addDays(todayWib(), days)
  const hh = (h: number) => String(h).padStart(2, '0')
  return {
    start_at: `${day}T${hh(startHour)}:00:00+07:00`,
    end_at: `${day}T${hh(endHour)}:00:00+07:00`,
  }
}

let counter = 0

export function makeBooking(over: Partial<Booking> = {}): Booking {
  counter += 1
  return {
    id: `booking-${counter}`,
    room_id: LAB.id,
    room: LAB,
    requester_id: sampleUser.id,
    requester: {
      id: sampleUser.id,
      identifier: sampleUser.identifier,
      name: sampleUser.name,
    },
    ...slotOn(2, 10, 12),
    purpose: 'Praktikum Basis Data',
    participants: 32,
    notes: '',
    details: {},
    status: 'MENUNGGU',
    approval_mode: 'MANUAL',
    manual_reason: 'ROOM_POLICY',
    decided_at: null,
    decision_reason: '',
    group_id: null,
    occurrence_no: null,
    occurrence_count: 1,
    expires_at: null,
    edited_at: null,
    letter: null,
    created_at: '2026-10-19T09:12:00+07:00',
    checked_in_at: null,
    checked_out_at: null,
    checkout_condition: null,
    ...over,
  } as Booking
}
