import { create } from 'zustand'

export type UploadStatus = 'uploading' | 'done' | 'error'

export interface UploadEntry {
  id: string
  name: string
  percent: number
  status: UploadStatus
}

interface UploadState {
  entries: UploadEntry[]
  add: (entry: UploadEntry) => void
  update: (id: string, patch: Partial<Omit<UploadEntry, 'id'>>) => void
  remove: (id: string) => void
  clear: () => void
}

export const useUploadStore = create<UploadState>((set) => ({
  entries: [],
  add: (entry) => set((s) => ({ entries: [...s.entries, entry] })),
  update: (id, patch) =>
    set((s) => ({
      entries: s.entries.map((e) => (e.id === id ? { ...e, ...patch } : e)),
    })),
  remove: (id) => set((s) => ({ entries: s.entries.filter((e) => e.id !== id) })),
  clear: () => set({ entries: [] }),
}))
