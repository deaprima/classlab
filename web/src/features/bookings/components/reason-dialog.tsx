import { useEffect, useMemo } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Textarea } from '@/components/ui/textarea'

// BR-04 and the API's limits (decisions.md, Phase 3).
export const REASON_MIN = 5
export const REASON_MAX = 500

function reasonSchema(required: boolean) {
  const text = z
    .string()
    .trim()
    .max(REASON_MAX, `Alasan maksimal ${REASON_MAX} karakter.`)
  return z.object({
    reason: required
      ? text.min(
          REASON_MIN,
          `Alasan wajib diisi, minimal ${REASON_MIN} karakter.`
        )
      : text.refine(
          (v) => v === '' || v.length >= REASON_MIN,
          `Alasan minimal ${REASON_MIN} karakter, atau kosongkan.`
        ),
  })
}

type ReasonValues = { reason: string }

/** Asks for the reason a decision needs; `required` follows BR-04 for reject and approved-cancel. */
export function ReasonDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmText,
  required,
  isPending,
  onSubmit,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description: React.ReactNode
  confirmText: string
  required: boolean
  isPending: boolean
  onSubmit: (reason: string) => void
}) {
  const schema = useMemo(() => reasonSchema(required), [required])
  const form = useForm<ReasonValues>({
    resolver: zodResolver(schema),
    defaultValues: { reason: '' },
  })

  useEffect(() => {
    if (!open) form.reset({ reason: '' })
  }, [open, form])

  return (
    <Dialog open={open} onOpenChange={(o) => !isPending && onOpenChange(o)}>
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription asChild>
            <div>{description}</div>
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form
            id='reason-form'
            noValidate
            onSubmit={form.handleSubmit((v) => onSubmit(v.reason.trim()))}
          >
            <FormField
              control={form.control}
              name='reason'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Alasan{required ? '' : ' (opsional)'}</FormLabel>
                  <FormControl>
                    <Textarea rows={3} maxLength={REASON_MAX} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </form>
        </Form>
        <DialogFooter>
          <Button
            variant='outline'
            disabled={isPending}
            onClick={() => onOpenChange(false)}
          >
            Batal
          </Button>
          <Button
            type='submit'
            form='reason-form'
            variant='destructive'
            disabled={isPending}
          >
            {isPending && <Loader2 className='animate-spin' />}
            {confirmText}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
