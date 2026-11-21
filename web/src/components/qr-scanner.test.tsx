import type jsQR from 'jsqr'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { render } from 'vitest-browser-react'
import { userEvent } from 'vitest/browser'
import { QrScanner } from './qr-scanner'

/** A real MediaStream (a canvas feed) so <video> can play it; `stop` is spied to check cleanup. */
function fakeStream() {
  const canvas = document.createElement('canvas')
  canvas.width = 640
  canvas.height = 480
  const context = canvas.getContext('2d')!
  const paint = () => {
    context.fillStyle = '#888'
    context.fillRect(0, 0, canvas.width, canvas.height)
  }
  paint()
  const stream = canvas.captureStream(15)
  const timer = setInterval(paint, 50)
  const stops = stream.getTracks().map((track) => vi.spyOn(track, 'stop'))
  return { stream, stops, done: () => clearInterval(timer) }
}

const getUserMedia = vi.spyOn(navigator.mediaDevices, 'getUserMedia')
const decode = vi.fn<typeof jsQR>()

afterEach(() => {
  getUserMedia.mockReset()
  decode.mockReset()
})

describe('QrScanner', () => {
  it('opens the rear camera when it appears', async () => {
    const fake = fakeStream()
    getUserMedia.mockResolvedValue(fake.stream)
    decode.mockReturnValue(null)
    const screen = await render(
      <QrScanner
        decode={decode}
        onDetect={vi.fn()}
        onClose={vi.fn()}
        roomCode='KLS-101'
      />
    )
    await expect
      .element(screen.getByText('Arahkan ke QR di pintu KLS-101'))
      .toBeVisible()
    expect(getUserMedia).toHaveBeenCalledWith(
      expect.objectContaining({
        video: expect.objectContaining({
          facingMode: { ideal: 'environment' },
        }),
      })
    )
    fake.done()
  })

  it('hands over the decoded text and releases the camera', async () => {
    const fake = fakeStream()
    getUserMedia.mockResolvedValue(fake.stream)
    decode.mockReturnValue({
      data: 'https://x.test/check-in?room=A&t=b',
    } as never)
    const onDetect = vi.fn()
    await render(
      <QrScanner decode={decode} onDetect={onDetect} onClose={vi.fn()} />
    )
    await vi.waitFor(() =>
      expect(onDetect).toHaveBeenCalledWith(
        'https://x.test/check-in?room=A&t=b'
      )
    )
    await vi.waitFor(() =>
      fake.stops.forEach((s) => expect(s).toHaveBeenCalled())
    )
    fake.done()
  })

  it('keeps scanning and explains when the code is rejected', async () => {
    const fake = fakeStream()
    getUserMedia.mockResolvedValue(fake.stream)
    decode.mockReturnValue({ data: 'not ours' } as never)
    const onDetect = vi.fn(() => 'Itu bukan QR check-in ClassLab.')
    const screen = await render(
      <QrScanner decode={decode} onDetect={onDetect} onClose={vi.fn()} />
    )
    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent('Itu bukan QR check-in ClassLab.')
    // Still live: the camera was not released.
    fake.stops.forEach((s) => expect(s).not.toHaveBeenCalled())
    fake.done()
  })

  it('releases the camera and calls onClose on "Tutup"', async () => {
    const fake = fakeStream()
    getUserMedia.mockResolvedValue(fake.stream)
    decode.mockReturnValue(null)
    const onClose = vi.fn()
    const screen = await render(
      <QrScanner decode={decode} onDetect={vi.fn()} onClose={onClose} />
    )
    await expect
      .element(screen.getByRole('button', { name: 'Tutup' }))
      .toBeVisible()
    await userEvent.click(screen.getByRole('button', { name: 'Tutup' }))
    expect(onClose).toHaveBeenCalled()
    fake.stops.forEach((s) => expect(s).toHaveBeenCalled())
    fake.done()
  })

  it('explains a denied permission and points to the phone camera app', async () => {
    getUserMedia.mockRejectedValue(new DOMException('x', 'NotAllowedError'))
    const screen = await render(
      <QrScanner decode={decode} onDetect={vi.fn()} onClose={vi.fn()} />
    )
    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent(/Izin kamera ditolak/)
    await expect
      .element(screen.getByRole('alert'))
      .toHaveTextContent(/aplikasi kamera bawaan ponsel/)
    await expect
      .element(screen.getByRole('button', { name: 'Coba lagi' }))
      .toBeVisible()
  })
})
