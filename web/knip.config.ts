import type { KnipConfig } from 'knip'

const config: KnipConfig = {
  ignore: ['src/components/ui/**', 'src/components/layout/app-title.tsx'],
  // Part of the agreed stack (brief.md section 2) though no screen uses them yet.
  ignoreDependencies: ['date-fns', '@tanstack/react-table'],
}

export default config
