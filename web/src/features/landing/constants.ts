export const NAV_LINKS = [
  { href: '#cara-kerja', label: 'Cara kerja' },
  { href: '#fitur', label: 'Fitur' },
  { href: '#peran', label: 'Peran' },
  { href: '#aturan', label: 'Aturan' },
] as const

/** Horizontal padding of every landing section: 120 px on a 1440 frame (docs/DESIGN.md 3), less on small screens. */
export const SECTION_X =
  'mx-auto w-full max-w-[1440px] px-5 sm:px-10 xl:px-[120px]'
