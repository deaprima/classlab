import { useId, useRef, useState } from 'react'
import { Upload } from 'lucide-react'
import { cn } from '@/lib/utils'

type DropzoneProps = {
  onFiles: (files: File[]) => void
  /** Same syntax as <input accept>, e.g. "application/pdf,image/jpeg". */
  accept?: string
  multiple?: boolean
  disabled?: boolean
  title?: string
  hint?: string
  className?: string
}

/** Figma `Dropzone` (Default / Active while dragging). Picks files only; validation is the caller's. */
export function Dropzone({
  onFiles,
  accept,
  multiple = false,
  disabled,
  title = 'Tarik berkas ke sini atau pilih dari perangkat',
  hint,
  className,
}: DropzoneProps) {
  const inputId = useId()
  const inputRef = useRef<HTMLInputElement>(null)
  const [active, setActive] = useState(false)

  const take = (list: FileList | null) => {
    const files = list ? Array.from(list) : []
    if (files.length > 0) onFiles(multiple ? files : files.slice(0, 1))
  }

  return (
    <div
      data-slot='dropzone'
      data-active={active || undefined}
      onDragOver={(e) => {
        if (disabled) return
        e.preventDefault()
        setActive(true)
      }}
      onDragLeave={() => setActive(false)}
      onDrop={(e) => {
        e.preventDefault()
        setActive(false)
        if (!disabled) take(e.dataTransfer.files)
      }}
      className={cn(
        'flex flex-col items-center gap-2 rounded-2xl border border-dashed px-4 py-8 text-center transition-colors',
        active
          ? 'border-line-accent bg-accent-subtle'
          : 'border-line-strong bg-subtle',
        disabled && 'bg-disabled opacity-70',
        className
      )}
    >
      <Upload aria-hidden className='size-5 text-fg-secondary' />
      <label htmlFor={inputId} className='type-card-title text-fg'>
        {title}
      </label>
      {hint && <p className='type-body-sm text-fg-secondary'>{hint}</p>}
      <input
        id={inputId}
        ref={inputRef}
        type='file'
        className='sr-only'
        accept={accept}
        multiple={multiple}
        disabled={disabled}
        onChange={(e) => {
          take(e.target.files)
          e.target.value = ''
        }}
      />
      <button
        type='button'
        disabled={disabled}
        onClick={() => inputRef.current?.click()}
        className='mt-1 inline-flex h-8 items-center rounded-full border border-line-strong bg-surface px-3 type-button hover:border-line-hover disabled:pointer-events-none disabled:text-fg-tertiary'
      >
        Pilih berkas
      </button>
    </div>
  )
}
