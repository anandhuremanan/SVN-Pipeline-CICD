// The app follows the Windows theme unless one is chosen in Settings.
export type Theme = 'system' | 'light' | 'dark'

const STORAGE_KEY = 'svn-builder-theme'

export function toTheme(value: unknown): Theme {
  return value === 'light' || value === 'dark' ? value : 'system'
}

// applyTheme switches the page colours. The choice is also kept in the browser store so the
// next launch can paint in the right theme before the saved settings have loaded.
export function applyTheme(theme: Theme) {
  if (theme === 'system') {
    delete document.documentElement.dataset.theme
  } else {
    document.documentElement.dataset.theme = theme
  }
  try {
    localStorage.setItem(STORAGE_KEY, theme)
  } catch {
    // The saved settings remain the source of truth.
  }
}

export function rememberedTheme(): Theme {
  try {
    return toTheme(localStorage.getItem(STORAGE_KEY))
  } catch {
    return 'system'
  }
}
