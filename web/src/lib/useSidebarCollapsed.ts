import { useState } from 'react'

const STORAGE_KEY = 'sidebar.collapsed'

/** Persists the sidebar expand/collapse state in localStorage so it survives page reloads. */
export function useSidebarCollapsed() {
  const [isCollapsed, setIsCollapsed] = useState(() => {
    if (typeof window === 'undefined') return false
    return localStorage.getItem(STORAGE_KEY) === '1'
  })

  const toggleCollapsed = () => {
    setIsCollapsed((prev) => {
      const next = !prev
      if (typeof window !== 'undefined') {
        localStorage.setItem(STORAGE_KEY, next ? '1' : '0')
      }
      return next
    })
  }

  return [isCollapsed, toggleCollapsed] as const
}
