import { useEffect, useState } from 'react'
import { AlertCircle, Printer, RefreshCw } from 'lucide-react'
import QRCode from 'qrcode'
import { getErrorMessage } from '@/lib/api-error'
import { type Room } from '@/lib/api-types'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { useRoomQr } from '@/features/rooms/api'
import { useRotateRoomQr } from './api'

/** Figma "TU – Dialog QR Ruang": the room's check-in QR, to print and stick on the door. */
export function RoomQrDialog({
  room,
  onOpenChange,
}: {
  room: Room | null
  onOpenChange: (open: boolean) => void
}) {
  return (
    <Dialog open={room !== null} onOpenChange={onOpenChange}>
      {room && (
        <RoomQrContent room={room} onClose={() => onOpenChange(false)} />
      )}
    </Dialog>
  )
}

function RoomQrContent({ room, onClose }: { room: Room; onClose: () => void }) {
  const qr = useRoomQr(room.id)
  const [image, setImage] = useState<string | null>(null)
  const [encodeFailed, setEncodeFailed] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const rotate = useRotateRoomQr({ onSuccess: () => setConfirming(false) })
  const url = qr.data?.url

  useEffect(() => {
    if (!url) return
    let current = true
    QRCode.toDataURL(url, { width: 480, margin: 1, errorCorrectionLevel: 'M' })
      .then((data) => {
        if (!current) return
        setImage(data)
        setEncodeFailed(false)
      })
      .catch(() => current && setEncodeFailed(true))
    return () => {
      current = false
    }
  }, [url])

  const failed = qr.isError || encodeFailed

  return (
    <DialogContent className='sm:max-w-md'>
      <DialogHeader>
        <DialogTitle>QR Check-in · {room.code}</DialogTitle>
        <DialogDescription>
          Cetak dan tempel di pintu ruang. Pengguna memindai dengan kamera
          ponsel.
        </DialogDescription>
      </DialogHeader>

      {failed && (
        <Alert variant='error'>
          <AlertCircle />
          <AlertDescription>
            {qr.isError
              ? getErrorMessage(qr.error)
              : 'QR tidak dapat dibuat. Coba lagi.'}
          </AlertDescription>
        </Alert>
      )}
      {!failed && (
        <div className='mx-auto grid size-52 place-items-center rounded-xl border bg-white p-2'>
          {image ? (
            <img
              src={image}
              alt={`Kode QR check-in ruang ${room.code}`}
              className='size-full'
            />
          ) : (
            <Skeleton className='size-full' />
          )}
        </div>
      )}
      <p className='type-body-sm text-fg-secondary'>
        Token berbeda tiap ruang. Ganti token bila QR bocor; QR lama langsung
        tidak berlaku.
      </p>

      <DialogFooter>
        <Button variant='outline' onClick={onClose}>
          Tutup
        </Button>
        <Button
          variant='outline'
          disabled={!image || rotate.isPending}
          onClick={() => setConfirming(true)}
        >
          <RefreshCw />
          Ganti token
        </Button>
        <Button disabled={!image} onClick={() => image && printQr(room, image)}>
          <Printer />
          Cetak
        </Button>
      </DialogFooter>

      <ConfirmDialog
        open={confirming}
        onOpenChange={(o) => !o && !rotate.isPending && setConfirming(false)}
        title={`Ganti token ${room.code}?`}
        desc={
          <p>
            QR yang sudah tertempel di pintu berhenti berfungsi sampai kamu
            mencetak dan menempel QR yang baru.
          </p>
        }
        confirmText={rotate.isPending ? 'Mengganti…' : 'Ganti token'}
        destructive
        isLoading={rotate.isPending}
        handleConfirm={() => rotate.mutate(room)}
      />
    </DialogContent>
  )
}

const escapeHtml = (text: string) =>
  text.replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`)

/** Prints just the QR and the room name from a hidden frame, so the app's own page is untouched. */
function printQr(room: Room, image: string) {
  const frame = document.createElement('iframe')
  frame.setAttribute('aria-hidden', 'true')
  frame.style.cssText = 'position:fixed;width:0;height:0;border:0;opacity:0'
  document.body.append(frame)
  const doc = frame.contentDocument
  const win = frame.contentWindow
  if (!doc || !win) {
    frame.remove()
    return
  }
  doc.open()
  doc.write(`<!doctype html><html><head><title>QR ${escapeHtml(room.code)}</title>
<style>body{font-family:system-ui,sans-serif;text-align:center;margin:0;padding:48px}
img{width:320px;height:320px}h1{font-size:40px;margin:24px 0 4px}p{font-size:18px;margin:0;color:#444}</style></head>
<body><img src="${image}" alt=""><h1>${escapeHtml(room.code)}</h1><p>${escapeHtml(room.name)}</p>
<p>Pindai untuk check-in</p></body></html>`)
  doc.close()
  win.addEventListener('afterprint', () => frame.remove())
  const img = doc.querySelector('img')
  const go = () => {
    win.focus()
    win.print()
  }
  if (img?.complete) go()
  else img?.addEventListener('load', go)
}
