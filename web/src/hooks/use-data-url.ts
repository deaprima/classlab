import { useEffect, useState } from 'react'

/**
 * A `data:` URL for showing a Blob in an <img>. Read asynchronously with FileReader, so there is
 * no object URL to revoke and nothing breaks when StrictMode re-runs the effect. Undefined while
 * reading or when there is no blob.
 */
export function useDataUrl(blob: Blob | undefined): string | undefined {
  const [state, setState] = useState<{ source: Blob; url: string }>()

  useEffect(() => {
    if (!blob) return
    let cancelled = false
    const reader = new FileReader()
    reader.onload = () => {
      if (!cancelled && typeof reader.result === 'string') {
        setState({ source: blob, url: reader.result })
      }
    }
    reader.readAsDataURL(blob)
    return () => {
      cancelled = true
      reader.abort()
    }
  }, [blob])

  return blob && state?.source === blob ? state.url : undefined
}
