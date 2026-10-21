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
  /** Last date the policy allows; its helper text is shown under the first row. */
  maxDate?: string
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

  return (
    <div className='grid gap-3'>
      {fields.map((row, i) => (
        <div key={row.id} className='grid items-start gap-3 sm:grid-cols-3'>
          <FormField
            control={form.control}
            name={`occurrences.${i}.date`}
            render={({ field }) => (
              <FormItem>
                <FormLabel>Tanggal{fields.length > 1 && ` ${i + 1}`}</FormLabel>
                <FormControl>
                  <Input type='date' min={minDate} max={maxDate} {...field} />
                </FormControl>
                {i === 0 && maxDateHint && (
                  <p className='type-body-sm text-fg-secondary'>
                    {maxDateHint}
                  </p>
                )}
                <FormMessage />
              </FormItem>
            )}
          />
          <TimeField
            name={`occurrences.${i}.start_time`}
            label='Mulai'
            options={times.slice(0, -1)}
          />
          <div className='flex items-start gap-2'>
            <div className='min-w-0 grow'>
              <TimeField
                name={`occurrences.${i}.end_time`}
                label='Selesai'
                options={times.slice(1)}
              />
            </div>
            {fields.length > 1 && (
              <Button
                type='button'
                variant='ghost'
                size='icon'
                className='mt-6 shrink-0'
                aria-label={`Hapus tanggal ${i + 1}`}
                onClick={() => remove(i)}
              >
                <X />
              </Button>
            )}
          </div>
        </div>
      ))}
      {listError && <p className='type-body-sm text-fg-error'>{listError}</p>}
      {fields.length < MAX_OCCURRENCES && (
        <Button
          type='button'
          variant='ghost'
          size='sm'
          className='justify-self-start'
          onClick={() => append(emptyOccurrence())}
        >
          <Plus />
          Tambah Tanggal
        </Button>
      )}
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
