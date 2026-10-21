import { useFieldArray, useFormContext } from 'react-hook-form'
import { slotTimes } from '@/config/booking-rules'
import { Plus, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  emptyOccurrence,
  MAX_OCCURRENCES,
  SLOT_MINUTES,
  type RequestFormValues,
} from './request-schema'

type OccurrenceFieldsProps = {
  hours: { open: string; close: string }
  minDate: string
  /** Last date the policy allows. */
  maxDate?: string
  /** Shown beside the "Tambah Tanggal" button, e.g. "Bisa dipilih sampai Jum, 20 Nov 2026". */
  maxDateHint?: string
}

/** One or more dates, each with its own start and end time (BR-27). */
export function OccurrenceFields({
  hours,
  minDate,
  maxDate,
  maxDateHint,
}: OccurrenceFieldsProps) {
  const form = useFormContext<RequestFormValues>()
  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: 'occurrences',
  })
  const times = slotTimes(hours.open, hours.close, SLOT_MINUTES)
  const listError = form.formState.errors.occurrences?.root?.message
  const many = fields.length > 1

  return (
    <div className='grid gap-4'>
      {fields.map((row, i) => (
        <div
          key={row.id}
          className={
            many
              ? 'grid grid-cols-2 items-start gap-3 sm:grid-cols-[minmax(0,1fr)_7.5rem_7.5rem_2.5rem]'
              : 'grid grid-cols-2 items-start gap-3 sm:grid-cols-[minmax(0,1fr)_7.5rem_7.5rem]'
          }
        >
          <FormField
            control={form.control}
            name={`occurrences.${i}.date`}
            render={({ field }) => (
              <FormItem className='col-span-2 sm:col-span-1'>
                <FormLabel>Tanggal{many && ` ${i + 1}`}</FormLabel>
                <FormControl>
                  <Input type='date' min={minDate} max={maxDate} {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
          <TimeField
            name={`occurrences.${i}.start_time`}
            label='Mulai'
            options={times.slice(0, -1)}
          />
          <TimeField
            name={`occurrences.${i}.end_time`}
            label='Selesai'
            options={times.slice(1)}
          />
          {many && (
            <Button
              type='button'
              variant='ghost'
              size='icon'
              className='col-span-2 mt-0 justify-self-end sm:col-span-1 sm:mt-6 sm:justify-self-auto'
              aria-label={`Hapus tanggal ${i + 1}`}
              onClick={() => remove(i)}
            >
              <X />
            </Button>
          )}
        </div>
      ))}

      {listError && <p className='type-body-sm text-fg-error'>{listError}</p>}

      <div className='flex flex-wrap items-center justify-between gap-x-4 gap-y-1'>
        {fields.length < MAX_OCCURRENCES ? (
          <Button
            type='button'
            variant='ghost'
            size='sm'
            className='-ms-3'
            onClick={() => append(emptyOccurrence())}
          >
            <Plus />
            Tambah Tanggal
          </Button>
        ) : (
          <span />
        )}
        {maxDateHint && (
          <p className='type-body-sm text-fg-secondary'>{maxDateHint}</p>
        )}
      </div>
    </div>
  )
}

function TimeField({
  name,
  label,
  options,
}: {
  name: `occurrences.${number}.start_time` | `occurrences.${number}.end_time`
  label: string
  options: string[]
}) {
  const form = useFormContext<RequestFormValues>()
  return (
    <FormField
      control={form.control}
      name={name}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{label}</FormLabel>
          <Select value={field.value} onValueChange={field.onChange}>
            <FormControl>
              <SelectTrigger className='w-full'>
                <SelectValue placeholder='--:--' />
              </SelectTrigger>
            </FormControl>
            <SelectContent>
              {options.map((t) => (
                <SelectItem key={t} value={t}>
                  {t}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}
