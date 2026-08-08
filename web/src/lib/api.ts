import { useAuthStore } from '@/store/auth'
import type { ApiResponse } from '@/lib/types'

// api sends an authenticated request and unwraps the ApiResponse envelope.
// Throws on error responses.
// For SPA: cookie is auto-sent. For external clients: fallback to Bearer token.
export async function api<T>(path: string, options?: RequestInit): Promise<T> {
  const { token } = useAuthStore.getState()

  const res = await fetch(path, {
    ...options,
    headers: {
      ...options?.headers,
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      // FormData sets its own multipart boundary; forcing JSON here breaks uploads.
      ...(!(options?.body instanceof FormData) ? { 'Content-Type': 'application/json' } : {}),
    },
    credentials: 'include',
  })

  if (!res.ok) {
    const body: ApiResponse = await res.json().catch(() => ({}))
    throw new Error(body.error || 'Request failed')
  }

  const body: ApiResponse<T> = await res.json()
  if (body.error) {
    throw new Error(body.error)
  }
  return body.data as T
}

export async function tenantApi<T>(
  path: string,
  orgSlug: string,
  options?: RequestInit,
): Promise<T> {
  return api<T>(path, {
    ...options,
    headers: {
      ...options?.headers,
      'X-Org-Slug': orgSlug,
    },
  })
}

// adminApi calls admin-only API endpoints (no X-Org-Slug header needed).
export async function adminApi<T>(path: string, options?: RequestInit): Promise<T> {
  return api<T>(path, options)
}
