import { createFileRoute, notFound } from '@tanstack/react-router'
import { DesignCatalog } from '@/features/design'

// Component catalog for development only; it is not reachable in a production build.
export const Route = createFileRoute('/design')({
  beforeLoad: () => {
    if (!import.meta.env.DEV) throw notFound()
  },
  component: DesignCatalog,
})
