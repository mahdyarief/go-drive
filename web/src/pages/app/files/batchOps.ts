import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { tenantApi } from '@/lib/api'

interface BatchMoveData {
  moved: number
}

interface BatchDeleteData {
  deleted: number
}

export interface BatchFileOps {
  selectMode: boolean
  selectedIds: string[]
  toggleSelectMode: () => void
  toggleSelect: (fileId: string) => void
  clearSelection: () => void
  moveOpen: boolean
  setMoveOpen: (open: boolean) => void
  moveFolderId: string
  setMoveFolderId: (value: string) => void
  deleteOpen: boolean
  setDeleteOpen: (open: boolean) => void
  movePending: boolean
  deletePending: boolean
  moveError: string | null
  deleteError: string | null
  moveFiles: (folderId: string) => void
  deleteFiles: () => void
}

const errorMessage = (error: Error | null): string | null => (error ? error.message : null)

// useBatchFileOps owns the select-multiple mode state and the batch
// move/delete mutations for the files page.
export function useBatchFileOps(orgSlug: string | undefined): BatchFileOps {
  const queryClient = useQueryClient()
  const [selectMode, setSelectMode] = useState(false)
  const [selectedIds, setSelectedIds] = useState<string[]>([])
  const [moveOpen, setMoveOpen] = useState(false)
  const [moveFolderId, setMoveFolderId] = useState('')
  const [deleteOpen, setDeleteOpen] = useState(false)

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['t', 'files', orgSlug] })
    queryClient.invalidateQueries({ queryKey: ['t', 'folders', orgSlug] })
    queryClient.invalidateQueries({ queryKey: ['t', 'usage', orgSlug] })
  }

  const moveMutation = useMutation({
    mutationFn: ({ ids, folderId }: { ids: string[]; folderId: string }) =>
      tenantApi<BatchMoveData>('/api/t/files/batch', orgSlug!, {
        method: 'PATCH',
        body: JSON.stringify({ ids, folderId }),
      }),
    onSuccess: () => {
      invalidate()
      setMoveOpen(false)
      setMoveFolderId('')
      setSelectedIds([])
      setSelectMode(false)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (ids: string[]) =>
      tenantApi<BatchDeleteData>('/api/t/files/batch', orgSlug!, {
        method: 'DELETE',
        body: JSON.stringify({ ids }),
      }),
    onSuccess: () => {
      invalidate()
      setDeleteOpen(false)
      setSelectedIds([])
      setSelectMode(false)
    },
  })

  const toggleSelectMode = () => {
    setSelectMode((v) => !v)
    setSelectedIds([])
  }

  const toggleSelect = (fileId: string) => {
    setSelectedIds((prev) => (prev.includes(fileId) ? prev.filter((id) => id !== fileId) : [...prev, fileId]))
  }

  const clearSelection = () => {
    setSelectedIds([])
    setSelectMode(false)
  }

  return {
    selectMode,
    selectedIds,
    toggleSelectMode,
    toggleSelect,
    clearSelection,
    moveOpen,
    setMoveOpen,
    moveFolderId,
    setMoveFolderId,
    deleteOpen,
    setDeleteOpen,
    movePending: moveMutation.isPending,
    deletePending: deleteMutation.isPending,
    moveError: errorMessage(moveMutation.error),
    deleteError: errorMessage(deleteMutation.error),
    moveFiles: (folderId: string) => moveMutation.mutate({ ids: selectedIds, folderId }),
    deleteFiles: () => deleteMutation.mutate(selectedIds),
  }
}
