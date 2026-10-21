import { afterEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { CameraCapture } from './camera-capture'

/** A real MediaStream (a canvas feed) so <video> can play it; `stop` is spied to check cleanup. */
function fakeStream() {
  const canvas = document.createElement('canvas')
  canvas.width = 1920
  canvas.height = 1080
  const context = canvas.getContext('2d')!
  const paint = () => {
    context.fillStyle = '#888'
    context.fillRect(0, 0, canvas.width, canvas.height)
  }
  paint()
  const stream = canvas.captureStream(15)
  // captureStream only emits frames when the canvas changes.
  const timer = setInterval(paint, 50)
  const stops = stream.getTracks().map((track) => {
    const stop = vi.spyOn(track, 'stop')
    return stop
  })
  return { stream, stops, done: () => clearInterval(timer) }
}

function domError(name: string) {
  return new DOMException('x', name)
}

const getUserMedia = vi.spyOn(navigator.mediaDevices, 'getUserMedia')

afterEach(() => {
  getUserMedia.mockReset()
})

async function renderCamera() {
  const onCapture = vi.fn()
  const onCancel = vi.fn()
  const screen = await render(
    <CameraCapture onCapture={onCapture} onCancel={onCancel} />
  )
  return { screen, onCapture, onCancel }
}

describe('CameraCapture', () => {
  it('does not touch the camera until the user asks', async () => {
    await renderCamera()
    expect(getUserMedia).not.toHaveBeenCalled()
  })

  it('requests the rear camera first and shows the preview', async () => {
    const fake = fakeStream()
    getUserMedia.mockResolvedValue(fake.stream)
    const { screen } = await renderCamera()
    await userEvent.click(screen.getByRole('button', { name: 'Buka kamera' }))
    await expect
      .element(screen.getByRole('button', { name: 'Jepret' }))
      .toBeVisible()
    expect(getUserMedia).toHaveBeenCalledWith({
      video: {
        facingMode: { ideal: 'environment' },
        width: { ideal: 1920 },
        height: { ideal: 1080 },
      },
      audio: false,
    })
    fake.done()
  })

  it('retries with plain video when the constraints cannot be met', async () => {
    const fake = fakeStream()
    getUserMedia
      .mockRejectedValueOnce(domError('OverconstrainedError'))
      .mockResolvedValueOnce(fake.stream)
    const { screen } = await renderCamera()
    await userEvent.click(screen.getByRole('button', { name: 'Buka kamera' }))
    await expect
      .element(screen.getByRole('button', { name: 'Jepret' }))
      .toBeVisible()
    expect(getUserMedia).toHaveBeenLastCalledWith({ video: true, audio: false })
    fake.done()
  })

  it('goes shoot, review, then hands the JPEG over and stops every track', async () => {
    const fake = fakeStream()
    getUserMedia.mockResolvedValue(fake.stream)
    const { screen, onCapture } = await renderCamera()
    await userEvent.click(screen.getByRole('button', { name: 'Buka kamera' }))
    const shutter = screen.getByRole('button', { name: 'Jepret' })
    await expect.element(shutter).toBeEnabled()
    // Wait for the first frame, or the video still reports 0x0.
    const video = document.querySelector('video')!
    await vi.waitFor(() => expect(video.videoWidth).toBeGreaterThan(0))
    await userEvent.click(shutter)

    await expect
      .element(screen.getByRole('dialog', { name: 'Tinjau foto' }))
      .toBeVisible()
    fake.stops.forEach((stop) => expect(stop).toHaveBeenCalled())

    await userEvent.click(screen.getByRole('button', { name: 'Gunakan foto' }))
    expect(onCapture).toHaveBeenCalledTimes(1)
    const blob: Blob = onCapture.mock.calls[0][0]
    expect(blob.type).toBe('image/jpeg')
    expect(blob.size).toBeGreaterThan(0)
    fake.done()
  })

  it('keeps the longest side of the photo at 1280 px', async () => {
    const fake = fakeStream()
    getUserMedia.mockResolvedValue(fake.stream)
    const { screen, onCapture } = await renderCamera()
    await userEvent.click(screen.getByRole('button', { name: 'Buka kamera' }))
    const video = document.querySelector('video')!
    await vi.waitFor(() => expect(video.videoWidth).toBeGreaterThan(0))
    await userEvent.click(screen.getByRole('button', { name: 'Jepret' }))
    await userEvent.click(screen.getByRole('button', { name: 'Gunakan foto' }))
    const bitmap = await createImageBitmap(onCapture.mock.calls[0][0])
    expect(Math.max(bitmap.width, bitmap.height)).toBeLessThanOrEqual(1280)
    fake.done()
  })

  it('reopens the camera on Ulangi', async () => {
    const first = fakeStream()
    const second = fakeStream()
    getUserMedia
      .mockResolvedValueOnce(first.stream)
      .mockResolvedValueOnce(second.stream)
    const { screen } = await renderCamera()
    await userEvent.click(screen.getByRole('button', { name: 'Buka kamera' }))
    const video = document.querySelector('video')!
    await vi.waitFor(() => expect(video.videoWidth).toBeGreaterThan(0))
    await userEvent.click(screen.getByRole('button', { name: 'Jepret' }))
    await userEvent.click(screen.getByRole('button', { name: 'Ulangi' }))
    await expect
      .element(screen.getByRole('button', { name: 'Jepret' }))
      .toBeVisible()
    expect(getUserMedia).toHaveBeenCalledTimes(2)
    first.done()
    second.done()
  })

  it('stops the stream on Batal and tells the parent', async () => {
    const fake = fakeStream()
    getUserMedia.mockResolvedValue(fake.stream)
    const { screen, onCancel } = await renderCamera()
    await userEvent.click(screen.getByRole('button', { name: 'Buka kamera' }))
    await expect
      .element(screen.getByRole('button', { name: 'Jepret' }))
      .toBeVisible()
    await userEvent.click(screen.getByRole('button', { name: 'Batal' }))
    fake.stops.forEach((stop) => expect(stop).toHaveBeenCalled())
    expect(onCancel).toHaveBeenCalled()
    await expect
      .element(screen.getByRole('button', { name: 'Buka kamera' }))
      .toBeVisible()
    fake.done()
  })

  it('stops the stream when unmounted while live', async () => {
    const fake = fakeStream()
    getUserMedia.mockResolvedValue(fake.stream)
    const { screen } = await renderCamera()
    await userEvent.click(screen.getByRole('button', { name: 'Buka kamera' }))
    await expect
      .element(screen.getByRole('button', { name: 'Jepret' }))
      .toBeVisible()
    await screen.unmount()
    fake.stops.forEach((stop) => expect(stop).toHaveBeenCalled())
    fake.done()
  })

  it('releases a stream that arrives after the component is gone', async () => {
    const fake = fakeStream()
    let resolve!: (stream: MediaStream) => void
    getUserMedia.mockReturnValue(
      new Promise<MediaStream>((r) => {
        resolve = r
      })
    )
    const { screen } = await renderCamera()
    await userEvent.click(screen.getByRole('button', { name: 'Buka kamera' }))
    await screen.unmount()
    resolve(fake.stream)
    await vi.waitFor(() =>
      fake.stops.forEach((stop) => expect(stop).toHaveBeenCalled())
    )
    fake.done()
  })

  it.each([
    ['NotAllowedError', 'Izin kamera ditolak', 'Coba lagi'],
    ['NotFoundError', 'Perangkat tidak memiliki kamera', null],
    ['NotReadableError', 'Kamera sedang dipakai aplikasi lain', 'Coba lagi'],
    ['AbortError', 'Kamera tidak bisa dibuka', 'Coba lagi'],
  ])('explains %s', async (name, title, action) => {
    getUserMedia.mockRejectedValue(domError(name))
    const { screen } = await renderCamera()
    await userEvent.click(screen.getByRole('button', { name: 'Buka kamera' }))
    await expect.element(screen.getByText(title)).toBeVisible()
    if (action) {
      await expect
        .element(screen.getByRole('button', { name: action }))
        .toBeVisible()
    } else {
      await expect
        .element(screen.getByRole('button', { name: 'Coba lagi' }))
        .not.toBeInTheDocument()
    }
  })

  it('lets the user try again after a denied permission', async () => {
    const fake = fakeStream()
    getUserMedia
      .mockRejectedValueOnce(domError('NotAllowedError'))
      .mockResolvedValueOnce(fake.stream)
    const { screen } = await renderCamera()
    await userEvent.click(screen.getByRole('button', { name: 'Buka kamera' }))
    await userEvent.click(screen.getByRole('button', { name: 'Coba lagi' }))
    await expect
      .element(screen.getByRole('button', { name: 'Jepret' }))
      .toBeVisible()
    fake.done()
  })
})
