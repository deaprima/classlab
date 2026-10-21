/** "1,2 MB", "340 KB", "800 B": sizes as shown next to an attached file (Indonesian decimal comma). */
export function formatFileSize(bytes: number): string {
  const kb = bytes / 1024
  if (kb < 1) return `${bytes} B`
  const mb = kb / 1024
  if (mb < 1) return `${Math.round(kb)} KB`
  return `${mb.toFixed(1).replace('.', ',')} MB`
}
