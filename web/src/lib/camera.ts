/** Shared by the photo camera and the QR scanner: both open the device camera with `getUserMedia`. */

export type Facing = 'environment' | 'user'

export type CameraFailure =
  'denied' | 'no-camera' | 'busy' | 'interrupted' | 'capture' | 'other'

/** getUserMedia needs a secure context; without it only the file-input fallback works. */
export function hasLiveCamera(): boolean {
  return (
    typeof navigator !== 'undefined' &&
    typeof window !== 'undefined' &&
    window.isSecureContext !== false &&
    typeof navigator.mediaDevices?.getUserMedia === 'function'
  )
}

export function failureOf(error: unknown): CameraFailure {
  const name = error instanceof DOMException ? error.name : ''
  if (name === 'NotAllowedError' || name === 'SecurityError') return 'denied'
  if (name === 'NotFoundError' || name === 'DevicesNotFoundError') {
    return 'no-camera'
  }
  if (name === 'NotReadableError' || name === 'TrackStartError') return 'busy'
  return 'other'
}

export async function openStream(facing: Facing): Promise<MediaStream> {
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

export function stopStream(stream: MediaStream | null) {
  stream?.getTracks().forEach((track) => track.stop())
}
