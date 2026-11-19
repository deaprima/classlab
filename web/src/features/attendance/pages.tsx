import { getRouteApi } from '@tanstack/react-router'
import { CheckIn } from './check-in-page'
import { CheckOut } from './check-out-page'

const checkInRoute = getRouteApi('/_authenticated/check-in')
const checkOutRoute = getRouteApi('/_authenticated/check-out/$id')

/** Route entry of `/check-in?room=&t=`: hands the QR values to the page. */
export function CheckInPage() {
  const { room, t } = checkInRoute.useSearch()
  return <CheckIn room={room} token={t} />
}

/** Route entry of `/check-out/$id`. */
export function CheckOutPage() {
  const { id } = checkOutRoute.useParams()
  return <CheckOut bookingId={id} />
}
