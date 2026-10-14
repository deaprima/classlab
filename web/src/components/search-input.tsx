import { useState } from 'react'
import { SearchIcon } from 'lucide-react'
import { Input } from '@/components/ui/input'

/** Text search that applies on Enter or blur, so each keystroke does not fire a request. */
export function SearchInput({
  value,
  onCommit,
  placeholder,
  label,
}: {
  value: string | undefined
  onCommit: (value: string | undefined) => void
  placeholder: string
  label: string
}) {
  const [draft, setDraft] = useState(value ?? '')
  const commit = () => {
    const v = draft.trim()
    if (v !== (value ?? '')) onCommit(v || undefined)
  }
  return (
    <div className='relative'>
      <SearchIcon
        className='absolute start-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground'
        aria-hidden
      />
      <Input
        type='search'
        aria-label={label}
        placeholder={placeholder}
        className='ps-8'
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => e.key === 'Enter' && commit()}
      />
    </div>
  )
}
