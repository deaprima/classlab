import { cn } from '@/lib/utils'
import { useTheme } from '@/context/theme-provider'
import { Card, CardContent } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Page } from '@/components/layout/page'

const THEMES = [
  { value: 'light', label: 'Terang' },
  { value: 'dark', label: 'Gelap' },
  { value: 'system', label: 'Ikuti sistem' },
] as const

/** Per-browser display preference; applied immediately and kept in localStorage. */
export function Appearance() {
  const { theme, setTheme } = useTheme()

  return (
    <Page
      title='Tampilan'
      description='Atur tema. Pengaturan ini hanya berlaku di browser ini.'
    >
      <Card className='max-w-xl'>
        <CardContent className='grid gap-3'>
          <Label id='theme-label'>Tema</Label>
          <RadioGroup
            aria-labelledby='theme-label'
            value={theme}
            onValueChange={(v) =>
              setTheme(v as (typeof THEMES)[number]['value'])
            }
            className='grid grid-cols-1 gap-3 sm:grid-cols-3'
          >
            {THEMES.map((t) => (
              <Label
                key={t.value}
                htmlFor={`theme-${t.value}`}
                className={cn(
                  'flex min-h-10 cursor-pointer items-center gap-2 rounded-md border p-3 font-normal',
                  theme === t.value && 'border-line-hover'
                )}
              >
                <RadioGroupItem id={`theme-${t.value}`} value={t.value} />
                {t.label}
              </Label>
            ))}
          </RadioGroup>
        </CardContent>
      </Card>
    </Page>
  )
}
