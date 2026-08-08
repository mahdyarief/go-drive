import { create } from 'zustand'

export type Theme = 'light' | 'dark' | 'system'

interface NotificationPrefs {
  shareActivity: boolean
  downloads: boolean
  newMembers: boolean
  storage: boolean
}

interface PreferencesState extends NotificationPrefs {
  theme: Theme
  setTheme: (theme: Theme) => void
  setNotificationPref: (key: keyof NotificationPrefs, value: boolean) => void
}

const THEME_KEY = 'theme'
const NOTIF_KEY = 'notificationPrefs'

const DEFAULT_PREFS: NotificationPrefs = {
  shareActivity: true,
  downloads: true,
  newMembers: true,
  storage: true,
}

const readStored = <T,>(key: string, fallback: T): T => {
  try {
    const raw = localStorage.getItem(key)
    return raw ? (JSON.parse(raw) as T) : fallback
  } catch {
    return fallback
  }
}

const storedPrefs = readStored<NotificationPrefs>(NOTIF_KEY, DEFAULT_PREFS)

// Applies the theme class to <html> — called from the appearance page and
// whenever the stored value changes (DOM classList is an external system).
export const applyTheme = (theme: Theme) => {
  const dark =
    theme === 'dark' ||
    (theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', dark)
}

export const usePreferencesStore = create<PreferencesState>((set) => ({
  theme: readStored<Theme>(THEME_KEY, 'light'),
  ...storedPrefs,

  setTheme: (theme) => {
    localStorage.setItem(THEME_KEY, JSON.stringify(theme))
    applyTheme(theme)
    set({ theme })
  },

  setNotificationPref: (key, value) => {
    const prefs: NotificationPrefs = {
      shareActivity: usePreferencesStore.getState().shareActivity,
      downloads: usePreferencesStore.getState().downloads,
      newMembers: usePreferencesStore.getState().newMembers,
      storage: usePreferencesStore.getState().storage,
      [key]: value,
    }
    localStorage.setItem(NOTIF_KEY, JSON.stringify(prefs))
    set({ [key]: value } as Partial<PreferencesState>)
  },
}))
