import { useAuthStore } from '@/store/auth'

export interface UploadedFile {
  name: string
  id: string
  size: number
}

export interface FailedUpload {
  name: string
  error: string
}

export interface UploadResult {
  files: UploadedFile[]
  failed: FailedUpload[]
}

// uploadFiles posts a batch of files to the multi-file upload endpoint via
// XHR so upload.onprogress can drive progress rows. Auth headers mirror api.ts.
export function uploadFiles(
  files: File[],
  orgSlug: string,
  folderId: string | null,
  onProgress: (percent: number) => void,
): Promise<UploadResult> {
  const { token } = useAuthStore.getState()

  const form = new FormData()
  for (const file of files) form.append('files', file)
  if (folderId) form.append('folderId', folderId)

  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', '/api/t/upload')
    xhr.withCredentials = true
    if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`)
    xhr.setRequestHeader('X-Org-Slug', orgSlug)

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && e.total > 0) {
        onProgress(Math.round((e.loaded / e.total) * 100))
      }
    }

    xhr.onload = () => {
      let body: { data?: UploadResult; error?: string }
      try {
        body = JSON.parse(xhr.responseText)
      } catch {
        reject(new Error('Request failed'))
        return
      }
      if (xhr.status >= 200 && xhr.status < 300 && body.data) {
        resolve(body.data)
      } else {
        reject(new Error(body.error || 'Request failed'))
      }
    }
    xhr.onerror = () => reject(new Error('Request failed'))
    xhr.send(form)
  })
}
