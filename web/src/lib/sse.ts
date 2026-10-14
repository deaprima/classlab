export type SseEvent = { event: string; data: string; id?: string }

/**
 * Incremental Server-Sent Events parser (WHATWG event-stream format). Feed it decoded text chunks
 * as they arrive; it calls `onEvent` for every complete event and ignores comment lines
 * (`: keep-alive`). Needed because EventSource cannot send an Authorization header.
 */
export function createSseParser(onEvent: (event: SseEvent) => void) {
  let buffer = ''
  let event = ''
  let data: string[] = []
  let id: string | undefined

  const dispatch = () => {
    if (data.length > 0) {
      onEvent({ event: event || 'message', data: data.join('\n'), id })
    }
    event = ''
    data = []
    id = undefined
  }

  const line = (raw: string) => {
    if (raw === '') return dispatch()
    if (raw.startsWith(':')) return
    const colon = raw.indexOf(':')
    const field = colon === -1 ? raw : raw.slice(0, colon)
    let value = colon === -1 ? '' : raw.slice(colon + 1)
    if (value.startsWith(' ')) value = value.slice(1)
    if (field === 'event') event = value
    else if (field === 'data') data.push(value)
    else if (field === 'id') id = value
  }

  return (chunk: string) => {
    buffer += chunk
    const lines = buffer.split(/\r\n|\r|\n/)
    // The last piece may be an incomplete line; keep it for the next chunk.
    buffer = lines.pop() ?? ''
    for (const l of lines) line(l)
  }
}
