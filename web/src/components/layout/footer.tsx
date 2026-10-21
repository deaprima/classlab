import { Link } from '@tanstack/react-router'
import { ACCESS, useHasRole } from '@/lib/rbac'

/** A one-line footer under every signed-in page: the copyright and, for requesters, the guide. */
export function Footer() {
  const requester = useHasRole(ACCESS.createBooking)
  return (
    <footer className='mt-auto flex flex-col items-center gap-1 px-4 pt-2 pb-5 text-center type-body-sm text-fg-secondary md:flex-row md:justify-between md:gap-x-4 md:px-6 md:text-start'>
      <span>© {new Date().getFullYear()} ClassLab.io</span>
      {requester && (
        <Link to='/guide' className='underline-offset-4 hover:underline'>
          Panduan
        </Link>
      )}
    </footer>
  )
}
