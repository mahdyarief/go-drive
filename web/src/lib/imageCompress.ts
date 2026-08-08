// Compress PNG/JPEG to WebP on the client so Drive storage stays small.
// Returns the original file untouched for PDFs, WebP, and other types.
export async function compressToWebP(file: File, maxDimension = 1600, quality = 0.8): Promise<File> {
  if (file.type !== 'image/png' && file.type !== 'image/jpeg') return file

  const bitmap = await loadBitmap(file)
  const scale = Math.min(1, maxDimension / Math.max(bitmap.width, bitmap.height))
  const canvas = document.createElement('canvas')
  canvas.width = Math.round(bitmap.width * scale)
  canvas.height = Math.round(bitmap.height * scale)
  const ctx = canvas.getContext('2d')
  if (!ctx) return file
  ctx.drawImage(bitmap, 0, 0, canvas.width, canvas.height)
  if ('close' in bitmap) bitmap.close()

  const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/webp', quality))
  if (!blob) return file

  const base = file.name.replace(/\.(png|jpe?g)$/i, '')
  return new File([blob], `${base}.webp`, { type: 'image/webp' })
}

async function loadBitmap(file: File): Promise<ImageBitmap | HTMLImageElement> {
  if ('createImageBitmap' in window) {
    try {
      return await createImageBitmap(file)
    } catch {
      // fall through to the <img> path for unsupported formats
    }
  }
  const url = URL.createObjectURL(file)
  try {
    return await new Promise<HTMLImageElement>((resolve, reject) => {
      const img = new Image()
      img.addEventListener('load', () => resolve(img), { once: true })
      img.addEventListener('error', () => reject(new Error('failed to load image')), { once: true })
      img.src = url
    })
  } finally {
    URL.revokeObjectURL(url)
  }
}
