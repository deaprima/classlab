import { describe, expect, it } from 'vitest'
import { createSseParser, type SseEvent } from './sse'

function parse(chunks: string[]) {
  const events: SseEvent[] = []
  const feed = createSseParser((e) => events.push(e))
  chunks.forEach(feed)
  return events
}

describe('createSseParser', () => {
  it('parses named events with ids and ignores keep-alive comments', () => {
    expect(
      parse([
        'event: ready\ndata: {"unread":2}\n\n',
        ': keep-alive\n\n',
        'id: abc\nevent: notification\ndata: {"id":"abc"}\n\n',
      ])
    ).toEqual([
      { event: 'ready', data: '{"unread":2}', id: undefined },
      { event: 'notification', data: '{"id":"abc"}', id: 'abc' },
    ])
  })

  it('reassembles events split across chunks, even mid-line', () => {
    expect(parse(['eve', 'nt: resync\nda', 'ta: {}', '\n', '\n'])).toEqual([
      { event: 'resync', data: '{}', id: undefined },
    ])
  })

  it('handles CRLF line endings and multi-line data', () => {
    expect(parse(['data: a\r\ndata: b\r\n\r\n'])).toEqual([
      { event: 'message', data: 'a\nb', id: undefined },
    ])
  })

  it('does not emit an event before its blank line arrives', () => {
    expect(parse(['event: ready\ndata: {}\n'])).toEqual([])
  })
})
