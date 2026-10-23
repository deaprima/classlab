// Apply the saved theme before first paint. Keep in sync with context/theme-provider.tsx.
// A file of its own (not an inline script) so the Content-Security-Policy can say script-src 'self'.
try {
  var t = localStorage.getItem('classlab-theme')
  var dark =
    t === 'dark' ||
    (t !== 'light' && matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.add(dark ? 'dark' : 'light')
} catch (e) {}
