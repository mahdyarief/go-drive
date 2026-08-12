import { create } from 'zustand'
import { uploadFiles } from '@/lib/upload'
import type { UploadResult } from '@/lib/upload'

export type UploadStatus = 'uploading' | 'done' | 'error'

export interface UploadEntry {
  id: string
  name: string
  percent: number
  status: UploadStatus
  error?: string
}

interface UploadState {
  entries: UploadEntry[]
  add: (entry: UploadEntry) => void
  update: (id: string, patch: Partial<Omit<UploadEntry, 'id'>>) => void
  remove: (id: string) => void
  clear: () => void
  uploadBatch: (files: File[], orgSlug: string, folderId: string | null) => Promise<UploadResult>
}

export const useUploadStore = create<UploadState>((set, get) => ({
  entries: [],
  add: (entry) => set((s) => ({ entries: [...s.entries, entry] })),
  update: (id, patch) =>
    set((s) => ({
      entries: s.entries.map((e) => (e.id === id ? { ...e, ...patch } : e)),
    })),
  remove: (id) => set((s) => ({ entries: s.entries.filter((e) => e.id !== id) })),
  clear: () => set({ entries: [] }),
  uploadBatch: async (files, orgSlug, folderId) => {
    const ids = files.map((f) => {
      const id = crypto.randomUUID()
      get().add({ id, name: f.name, percent: 0, status: 'uploading' })
      return { id, name: f.name }
    })
    const update = get().update
    try {
      const res = await uploadFiles(files, orgSlug, folderId, (percent) => {
        ids.forEach(({ id }) => update(id, { percent }))
      })
      res.files.forEach((f) => {
        const entry = ids.find((u) => u.name === f.name)
        if (entry) update(entry.id, { percent: 100, status: 'done' })
      })
      res.failed.forEach((f) => {
        const entry = ids.find((u) => u.name === f.name)
        if (entry) update(entry.id, { status: 'error', error: f.error })
      })
      return res
    } catch (err) {
      ids.forEach(({ id }) => update(id, { status: 'error' }))
      throw err
    }
  },
}))
