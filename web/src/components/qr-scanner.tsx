import { useCallback, useEffect, useRef, useState } from 'react'
import jsQR from 'jsqr'
import { Loader2, QrCode, X } from 'lucide-react'
import {
  type CameraFailure,
  failureOf,
  hasLiveCamera,
  openStream,
  stopStream,
} from '@/lib/camera'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'

/** The frame is shrunk before decoding: a QR on a door is large, and a small image is much cheaper to scan. */
const DECODE_WIDTH = 640
/** Decode at most this often; ~8 scans a second is plenty and keeps phones cool. */
const SCAN_INTERVAL_MS = 120

const FALLBACK = 'Atau pindai QR di pintu dengan aplikasi kamera bawaan ponsel.'

const FAILURE_COPY: Record<CameraFailure, { title: string; body: string }> = {
  denied: {
    title: 'Izin kamera ditolak',
    body: `Izinkan Kamera untuk situs ini di pengaturan browser, lalu coba lagi. ${FALLBACK}`,
  },
  'no-camera': {
    title: 'Perangkat tidak memiliki kamera',
    body: `Scanner butuh kamera. ${FALLBACK}`,
  },
  busy: {
    title: 'Kamera sedang dipakai aplikasi lain',
    body: 'Tutup aplikasi atau tab yang memakai kamera, lalu coba lagi.',
  },
  interrupted: {
    title: 'Kamera dihentikan',
    body: 'Kamera mati karena tab ini ditinggalkan. Buka scanner lagi untuk melanjutkan.',
  },
  capture: {
    title: 'Kamera belum siap',
    body: 'Tunggu gambar tampil, lalu arahkan ke QR.',
  },
  other: {
    title: 'Kamera tidak bisa dibuka',
    body: `Muat ulang halaman dan coba lagi. ${FALLBACK}`,
  },
}

type QrScannerProps = {
  /**
   * Called with the decoded text. Return a message to reject it (the scanner keeps looking and
   * shows the message), or nothing to accept it, which closes the camera.
   */
  onDetect: (text: string) => string | void
  onClose: () => void
  /** Names the room being checked in to, so the person knows which door QR to aim at. */
  roomCode?: string
  /** Reads a QR out of pixels; jsQR unless a test supplies its own. */
  decode?: typeof jsQR
}

/**
 * Full-screen QR scanner on the device camera. The camera starts when this mounts (the tap that
 * opened it is the user's intent) and every track stops on accept, close or unmount. Decoding is
 * done in JavaScript (jsQR) because `BarcodeDetector` is missing on iOS Safari. Where the camera
 * cannot open (plain HTTP, no permission) the person is pointed to the phone's own camera app,
 * which opens the same `/check-in` link.
 */
export function QrScanner({
  onDetect,
  onClose,
  roomCode,
  decode = jsQR,
}: QrScannerProps) {
  const [failure, setFailure] = useState<CameraFailure | null>(
    hasLiveCamera() ? null : 'other'
  )
  const [ready, setReady] = useState(false)
  const [rejected, setRejected] = useState<string | null>(null)
  const videoRef = useRef<HTMLVideoElement>(null)
  const canvasRef = useRef<HTMLCanvasElement | null>(null)
  const streamRef = useRef<MediaStream | null>(null)
  const frameRef = useRef(0)
  // Bumped on every start/stop, so a getUserMedia that resolves late can tell it was abandoned.
  const attempt = useRef(0)
  const onDetectRef = useRef(onDetect)
  const decodeRef = useRef(decode)
  useEffect(() => {
    onDetectRef.current = onDetect
    decodeRef.current = decode
  }, [onDetect, decode])

  const stop = useCallback(() => {
    attempt.current += 1
    cancelAnimationFrame(frameRef.current)
    stopStream(streamRef.current)
    streamRef.current = null
  }, [])

  const start = useCallback(async () => {
    stop()
    const mine = attempt.current
    setFailure(null)
    setReady(false)
    setRejected(null)
    if (!hasLiveCamera()) {
      setFailure('other')
      return
    }
    try {
      const stream = await openStream('environment')
      if (mine !== attempt.current) {
        stopStream(stream)
        return
      }
      streamRef.current = stream
      const video = videoRef.current
      if (!video) return
      video.srcObject = stream
      await video.play().catch(() => {})
      if (mine !== attempt.current) return

      const canvas = (canvasRef.current ??= document.createElement('canvas'))
      const context = canvas.getContext('2d', { willReadFrequently: true })
      let last = 0
      const tick = (time: number) => {
        if (mine !== attempt.current) return
        frameRef.current = requestAnimationFrame(tick)
        if (time - last < SCAN_INTERVAL_MS || video.videoWidth === 0) return
        last = time
        setReady(true)
        if (!context) return
        const scale = Math.min(1, DECODE_WIDTH / video.videoWidth)
        canvas.width = Math.round(video.videoWidth * scale)
        canvas.height = Math.round(video.videoHeight * scale)
        context.drawImage(video, 0, 0, canvas.width, canvas.height)
        const image = context.getImageData(0, 0, canvas.width, canvas.height)
        const code = decodeRef.current(image.data, image.width, image.height, {
          inversionAttempts: 'dontInvert',
        })
        if (!code?.data) return
        const problem = onDetectRef.current(code.data)
        if (problem) {
          setRejected(problem)
          return
        }
        stop()
      }
      frameRef.current = requestAnimationFrame(tick)
    } catch (error) {
      if (mine !== attempt.current) return
      setFailure(failureOf(error))
    }
  }, [stop])

  useEffect(() => {
    // The camera is an external system: opening it on mount is the effect.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void start()
    return stop
  }, [start, stop])

  // A hidden tab may lose the camera anyway; release it and ask to open it again.
  useEffect(() => {
    const onHide = () => {
      if (document.visibilityState !== 'hidden' || !streamRef.current) return
      stop()
      setFailure('interrupted')
    }
    document.addEventListener('visibilitychange', onHide)
    return () => document.removeEventListener('visibilitychange', onHide)
  }, [stop])

  return (
    <div
      role='dialog'
      aria-modal='true'
      aria-label='Pindai QR check-in'
      className='fixed inset-0 z-50 flex h-dvh flex-col bg-black text-white'
    >
      <div className='relative min-h-0 flex-1'>
        {/* muted + playsInline: iOS Safari refuses inline autoplay otherwise. */}
        <video
          ref={videoRef}
          autoPlay
          playsInline
          muted
          aria-label='Pratinjau kamera'
          className='size-full object-cover'
        />
        {!failure && (
          <div className='pointer-events-none absolute inset-0 flex flex-col items-center justify-center gap-4'>
            <div
              aria-hidden
              className='size-64 max-w-[70vw] rounded-3xl border-4 border-white/90 shadow-[0_0_0_9999px_rgba(0,0,0,0.45)]'
            />
            {!ready ? (
              <p className='flex items-center gap-2'>
                <Loader2 className='animate-spin' aria-hidden />
                Membuka kamera...
              </p>
            ) : (
              <p className='rounded-full bg-black/60 px-4 py-2 text-center'>
                {roomCode
                  ? `Arahkan ke QR di pintu ${roomCode}`
                  : 'Arahkan ke QR di pintu ruang'}
              </p>
            )}
          </div>
        )}
        {(failure || rejected) && (
          <div className='absolute inset-x-4 top-4'>
            <Alert variant='error'>
              <QrCode />
              <AlertTitle>
                {failure ? FAILURE_COPY[failure].title : 'QR tidak dikenali'}
              </AlertTitle>
              <AlertDescription>
                {failure ? FAILURE_COPY[failure].body : rejected}
              </AlertDescription>
            </Alert>
          </div>
        )}
      </div>
      <div className='flex items-center justify-between gap-4 px-6 pt-4 pb-[max(1.5rem,env(safe-area-inset-bottom))]'>
        <Button
          type='button'
          variant='ghost'
          className='h-11 text-white hover:bg-white/15 hover:text-white'
          onClick={() => {
            stop()
            onClose()
          }}
        >
          <X />
          Tutup
        </Button>
        {failure && failure !== 'no-camera' && (
          <Button type='button' className='h-11' onClick={() => void start()}>
            Coba lagi
          </Button>
        )}
      </div>
    </div>
  )
}
