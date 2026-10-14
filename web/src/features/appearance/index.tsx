import { fonts } from '@/config/fonts'
import { cn } from '@/lib/utils'
import { useFont } from '@/context/font-provider'
import { useTheme } from '@/context/theme-provider'
import { Card, CardContent } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Page } from '@/components/layout/page'

const THEMES = [
  { value: 'light', label: 'Terang' },
  { value: 'dark', label: 'Gelap' },
  { value: 'system', label: 'Ikuti sistem' },
] as const

/** Per-browser display preferences; applied immediately and kept in a cookie by the providers. */
export function Appearance() {
  const { font, setFont } = useFont()
  const { theme, setTheme } = useTheme()

  return (
    <Page
      title='Tampilan'
      description='Atur tema dan huruf. Pengaturan ini hanya berlaku di browser ini.'
    >
      <Card className='max-w-xl'>
        <CardContent className='grid gap-8'>
          <div className='grid gap-3'>
            <Label id='theme-label'>Tema</Label>
            <RadioGroup
              aria-labelledby='theme-label'
              value={theme}
              onValueChange={(v) =>
                setTheme(v as (typeof THEMES)[number]['value'])
              }
              className='grid grid-cols-3 gap-3'
            >
              {THEMES.map((t) => (
                <Label
                  key={t.value}
                  htmlFor={`theme-${t.value}`}
                  className={cn(
                    'flex cursor-pointer items-center gap-2 rounded-md border p-3 font-normal',
                    theme === t.value && 'border-primary'
                  )}
                >
                  <RadioGroupItem id={`theme-${t.value}`} value={t.value} />
                  {t.label}
                </Label>
              ))}
            </RadioGroup>
          </div>
          <div className='grid gap-3'>
            <Label htmlFor='font'>Huruf</Label>
            <Select
              value={font}
              onValueChange={(v) => setFont(v as typeof font)}
            >
              <SelectTrigger id='font' className='w-56 capitalize'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {fonts.map((f) => (
                  <SelectItem key={f} value={f} className='capitalize'>
                    {f}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </CardContent>
      </Card>
    </Page>
  )
}
