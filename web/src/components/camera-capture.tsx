import { useCallback, useEffect, useRef, useState } from 'react'
import { Camera, Loader2, RefreshCw, SwitchCamera, X } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useDataUrl } from '@/hooks/use-data-url'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'

/** Longest side of the exported photo; keeps uploads small without hurting legibility. */
const MAX_SIDE = 1280
const JPEG_QUALITY = 0.8

type Facing = 'environment' | 'user'

type Failure =
  | 'denied'
  | 'no-camera'
  | 'busy'
  | 'interrupted'
  | 'capture'
  | 'other'

type Status =
  | { kind: 'idle' }
  | { kind: 'starting' }
  | { kind: 'live' }
  | { kind: 'review'; blob: Blob }
  | { kind: 'failed'; failure: Failure }

const FAILURE_COPY: Record<Failure, { title: string; body: string }> = {
  denied: {
    title: 'Izin kamera ditolak',
    body: 'Buka pengaturan situs di browser (ikon gembok di kolom alamat), izinkan Kamera untuk situs ini, lalu coba lagi.',
  },
  'no-camera': {
    title: 'Perangkat tidak memiliki kamera',
    body: 'Check-out butuh foto langsung dari kamera. Gunakan perangkat yang punya kamera.',
  },
  busy: {
    title: 'Kamera sedang dipakai aplikasi lain',
    body: 'Tutup aplikasi atau tab yang memakai kamera, lalu coba lagi.',
  },
  interrupted: {
    title: 'Kamera dihentikan',
    body: 'Kamera mati karena tab ini ditinggalkan. Buka kamera lagi untuk melanjutkan.',
  },
  capture: {
    title: 'Foto belum bisa diambil',
    body: 'Kamera belum siap. Tunggu gambar tampil, lalu jepret lagi.',
  },
  other: {
    title: 'Kamera tidak bisa dibuka',
    body: 'Muat ulang halaman dan coba lagi. Bila tetap gagal, pastikan alamat situs memakai HTTPS.',
  },
}

/** getUserMedia needs a secure context; without it only the file-input fallback works. */
function hasLiveCamera(): boolean {
  return (
    typeof navigator !== 'undefined' &&
    typeof window !== 'undefined' &&
    window.isSecureContext !== false &&
    typeof navigator.mediaDevices?.getUserMedia === 'function'
  )
}

function failureOf(error: unknown): Failure {
  const name = error instanceof DOMException ? error.name : ''
  if (name === 'NotAllowedError' || name === 'SecurityError') return 'denied'
  if (name === 'NotFoundError' || name === 'DevicesNotFoundError') {
    return 'no-camera'
  }
  if (name === 'NotReadableError' || name === 'TrackStartError') return 'busy'
  return 'other'
}

async function openStream(facing: Facing): Promise<MediaStream> {
  try {
    return await navigator.mediaDevices.getUserMedia({
      video: {
        facingMode: { ideal: facing },
        width: { ideal: 1920 },
        height: { ideal: 1080 },
      },
      audio: false,
    })
  } catch (error) {
    if (
      error instanceof DOMException &&
      error.name === 'OverconstrainedError'
    ) {
      return navigator.mediaDevices.getUserMedia({ video: true, audio: false })
    }
    throw error
  }
}

function stopStream(stream: MediaStream | null) {
  stream?.getTracks().forEach((track) => track.stop())
}

function scaledSize(width: number, height: number) {
  const scale = Math.min(1, MAX_SIDE / Math.max(width, height))
  return {
    width: Math.round(width * scale),
    height: Math.round(height * scale),
  }
}

function toJpeg(video: HTMLVideoElement): Promise<Blob | null> {
  const { width, height } = scaledSize(video.videoWidth, video.videoHeight)
  const canvas = document.createElement('canvas')
  canvas.width = width
  canvas.height = height
  const context = canvas.getContext('2d')
  if (!context) return Promise.resolve(null)
  context.drawImage(video, 0, 0, width, height)
  return new Promise((resolve) =>
    canvas.toBlob(resolve, 'image/jpeg', JPEG_QUALITY)
  )
}

type CameraCaptureProps = {
  /** Called with the photo once the user accepts it ("Gunakan foto"). */
  onCapture: (photo: Blob) => void
  /** Called when the user backs out of the live camera or the review. */
  onCancel?: () => void
  className?: string
}

/**
 * Takes one photo with the device camera (brief 7.3). The camera starts only after a tap, every
 * track is stopped when the photo is taken, cancelled or the component unmounts, and the result
 * stays an in-memory Blob. Where `getUserMedia` is missing (plain HTTP, old browsers) a native
 * `<input capture>` stands in. This is a UX guard, not proof of origin: the server never trusts
 * the client (BR-16).
 */
export function CameraCapture({
  onCapture,
  onCancel,
  className,
}: CameraCaptureProps) {
  const live = hasLiveCamera()
  const [status, setStatus] = useState<Status>({ kind: 'idle' })
  const [facing, setFacing] = useState<Facing>('environment')
  const [canSwitch, setCanSwitch] = useState(false)
  const streamRef = useRef<MediaStream | null>(null)
  const videoRef = useRef<HTMLVideoElement>(null)
  // Bumped on every start/stop, so a getUserMedia that resolves late can tell it was abandoned.
  const attempt = useRef(0)

  const stop = useCallback(() => {
    attempt.current += 1
    stopStream(streamRef.current)
    streamRef.current = null
  }, [])

  // Unmounting must always release the camera.
  useEffect(() => stop, [stop])

  const start = useCallback(
    async (wanted: Facing) => {
      stop()
      const mine = attempt.current
      setStatus({ kind: 'starting' })
      try {
        const stream = await openStream(wanted)
        if (mine !== attempt.current) {
          stopStream(stream)
          return
        }
        streamRef.current = stream
        setFacing(wanted)
        setStatus({ kind: 'live' })
        const devices = await navigator.mediaDevices
          .enumerateDevices?.()
          .catch(() => [])
        if (mine === attempt.current) {
          setCanSwitch(
            (devices ?? []).filter((d) => d.kind === 'videoinput').length > 1
          )
        }
      } catch (error) {
        if (mine !== attempt.current) return
        setStatus({ kind: 'failed', failure: failureOf(error) })
      }
    },
    [stop]
  )

  // Hand the stream to the <video> once it is on screen.
  const showing = status.kind === 'live'
  useEffect(() => {
    const video = videoRef.current
    if (!showing || !video || !streamRef.current) return
    video.srcObject = streamRef.current
    video.play().catch(() => {})
  }, [showing])

  // A hidden tab may lose the camera anyway; release it and ask to open it again.
  useEffect(() => {
    if (!showing) return
    const onHide = () => {
      if (document.visibilityState !== 'hidden') return
      stop()
      setStatus({ kind: 'failed', failure: 'interrupted' })
    }
    document.addEventListener('visibilitychange', onHide)
    return () => document.removeEventListener('visibilitychange', onHide)
  }, [showing, stop])

  async function shoot() {
    const video = videoRef.current
    if (!video || video.videoWidth === 0) {
      setStatus({ kind: 'failed', failure: 'capture' })
      return
    }
    const blob = await toJpeg(video)
    if (!blob) {
      setStatus({ kind: 'failed', failure: 'capture' })
      return
    }
    stop()
    setStatus({ kind: 'review', blob })
  }

  function cancel() {
    stop()
    setStatus({ kind: 'idle' })
    onCancel?.()
  }

  if (status.kind === 'live' || status.kind === 'starting') {
    return (
      <div
        role='dialog'
        aria-modal='true'
        aria-label='Kamera'
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
            className='size-full object-contain'
          />
          {status.kind === 'starting' && (
            <div className='absolute inset-0 flex items-center justify-center gap-2'>
              <Loader2 className='animate-spin' aria-hidden />
              Membuka kamera...
            </div>
          )}
        </div>
        <div className='flex items-center justify-between gap-4 px-6 pt-4 pb-[max(1.5rem,env(safe-area-inset-bottom))]'>
          <Button
            type='button'
            variant='ghost'
            className='h-11 text-white hover:bg-white/15 hover:text-white'
            onClick={cancel}
          >
            <X />
            Batal
          </Button>
          <button
            type='button'
            onClick={shoot}
            disabled={status.kind !== 'live'}
            aria-label='Jepret'
            className='flex size-18 items-center justify-center rounded-full border-4 border-white bg-white/20 outline-none focus-visible:ring-4 focus-visible:ring-ring disabled:opacity-40'
          >
            <span className='size-12 rounded-full bg-white' />
          </button>
          {canSwitch ? (
            <Button
              type='button'
              variant='ghost'
              className='h-11 text-white hover:bg-white/15 hover:text-white'
              onClick={() =>
                start(facing === 'environment' ? 'user' : 'environment')
              }
            >
              <SwitchCamera />
              Ganti kamera
            </Button>
          ) : (
            <span className='w-24' aria-hidden />
          )}
        </div>
      </div>
    )
  }

  if (status.kind === 'review') {
    return (
      <Review
        blob={status.blob}
        onRetake={() => {
          setStatus({ kind: 'idle' })
          if (live) void start(facing)
        }}
        onUse={() => onCapture(status.blob)}
      />
    )
  }

  const failure = status.kind === 'failed' ? status.failure : null

  return (
    <div className={cn('flex flex-col gap-3', className)}>
      {failure && (
        <Alert variant='error' role='alert'>
          <Camera />
          <AlertTitle>{FAILURE_COPY[failure].title}</AlertTitle>
          <AlertDescription>{FAILURE_COPY[failure].body}</AlertDescription>
        </Alert>
      )}
      {live ? (
        failure !== 'no-camera' && (
          <Button type='button' className='h-11' onClick={() => start(facing)}>
            <Camera />
            {failure ? 'Coba lagi' : 'Buka kamera'}
          </Button>
        )
      ) : (
        <FileFallback
          onPick={(file) => setStatus({ kind: 'review', blob: file })}
        />
      )}
      <p className='type-body-sm text-fg-secondary'>
        Foto hanya dipakai untuk catatan kondisi ruang dan tidak disimpan ke
        galeri perangkatmu.
      </p>
    </div>
  )
}

function FileFallback({ onPick }: { onPick: (file: File) => void }) {
  return (
    <div className='flex flex-col gap-2'>
      <Alert variant='info'>
        <AlertDescription>
          Kamera langsung tidak tersedia di koneksi ini. Ambil foto dengan
          aplikasi kamera perangkat.
        </AlertDescription>
      </Alert>
      <Button type='button' className='h-11' asChild>
        <label>
          <Camera />
          Ambil foto
          {/* `capture` opens the camera app on phones instead of the gallery. */}
          <input
            type='file'
            accept='image/*'
            capture='environment'
            className='sr-only'
            onChange={(event) => {
              const file = event.target.files?.[0]
              if (file) onPick(file)
              event.target.value = ''
            }}
          />
        </label>
      </Button>
    </div>
  )
}

function Review({
  blob,
  onRetake,
  onUse,
}: {
  blob: Blob
  onRetake: () => void
  onUse: () => void
}) {
  const url = useDataUrl(blob)
  return (
    <div
      role='dialog'
      aria-modal='true'
      aria-label='Tinjau foto'
      className='fixed inset-0 z-50 flex h-dvh flex-col bg-black text-white'
    >
      <div className='min-h-0 flex-1'>
        {url && (
          <img
            src={url}
            alt='Foto kondisi ruang yang baru diambil'
            className='size-full object-contain'
          />
        )}
      </div>
      <div className='flex gap-3 px-4 pt-4 pb-[max(1rem,env(safe-area-inset-bottom))]'>
        <Button
          type='button'
          variant='secondary'
          className='h-11 flex-1'
          onClick={onRetake}
        >
          <RefreshCw />
          Ulangi
        </Button>
        <Button type='button' className='h-11 flex-1' onClick={onUse}>
          Gunakan foto
        </Button>
      </div>
    </div>
  )
}
