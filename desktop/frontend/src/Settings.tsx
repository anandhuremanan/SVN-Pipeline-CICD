import { useState } from 'react'
import type { ReactNode } from 'react'
import { ChooseFolder, ClearSavedPackages, SetPackageKeepDays, SetStorageDir } from '../wailsjs/go/main/App'
import type { main } from '../wailsjs/go/models'
import type { Theme } from './theme'
import { BackIcon, CheckIcon, FolderIcon, Notice, Spinner, fieldButton, primaryButton } from './ui'

// Below this much free space a drive is marked as nearly full.
const LOW_SPACE_BYTES = 2 * 1024 ** 3
const FULL_BYTES = 20 * 1024 ** 2

export function formatBytes(bytes: number) {
  const gb = bytes / 1024 ** 3
  if (gb >= 100) return `${Math.round(gb)} GB`
  if (gb >= 1) return `${gb.toFixed(1)} GB`
  return `${Math.floor(bytes / 1024 ** 2)} MB`
}

const samePath = (a: string, b: string) => a.replace(/[\\/]+$/, '').toLowerCase() === b.replace(/[\\/]+$/, '').toLowerCase()

// How long saved packages stay after a project's last build; 0 days means until cleared by hand.
const KEEP_OPTIONS = [
  { days: 3, label: '3 days' },
  { days: 7, label: '7 days' },
  { days: 30, label: '30 days' },
  { days: 0, label: 'Always' },
]

const THEMES: { value: Theme; label: string }[] = [
  { value: 'system', label: 'Same as Windows' },
  { value: 'light', label: 'Light' },
  { value: 'dark', label: 'Dark' },
]

export function Settings(props: {
  storage: main.StorageInfo
  onChange: (storage: main.StorageInfo) => void
  theme: Theme
  onThemeChange: (theme: Theme) => void
  onClose: () => void
}) {
  const { storage } = props
  const [busy, setBusy] = useState<string | null>(null)
  const [error, setError] = useState('')

  async function use(folder: string) {
    setError('')
    setBusy(folder)
    try {
      props.onChange(await SetStorageDir(folder))
    } catch (err) {
      setError(String(err))
    } finally {
      setBusy(null)
    }
  }

  const [clearing, setClearing] = useState(false)

  async function clearPackages() {
    setError('')
    setClearing(true)
    try {
      props.onChange(await ClearSavedPackages())
    } catch (err) {
      setError(String(err))
    } finally {
      setClearing(false)
    }
  }

  async function keepFor(days: number) {
    setError('')
    try {
      props.onChange(await SetPackageKeepDays(days))
    } catch (err) {
      setError(String(err))
    }
  }

  async function chooseOther() {
    const chosen = await ChooseFolder(storage.folder || storage.workspaceDir, 'Choose a working folder')
    if (chosen) await use(chosen)
  }

  const onDrive = storage.drives.some((d) => samePath(d.folder, storage.folder))
  const otherFolder = storage.folder !== '' && !onDrive
  const autoFree = Math.min(storage.autoWorkspaceFreeBytes, storage.autoDataFreeBytes)

  return (
    <div className="flex h-full flex-col">
      <main className="view-enter flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto p-5 pb-4">
        <header className="flex items-center gap-2 pb-1">
          <button
            type="button"
            onClick={props.onClose}
            aria-label="Back"
            title="Back"
            className="grid size-9 place-items-center rounded-lg text-muted transition hover:bg-surface hover:text-ink"
          >
            <BackIcon />
          </button>
          <h1 className="font-display text-[18px] leading-tight font-semibold">Settings</h1>
        </header>

        <section aria-labelledby="appearance" className="rounded-xl border border-line bg-surface px-4 py-3.5 shadow-card">
          <h2 id="appearance" className="text-[14px] font-semibold">
            Appearance
          </h2>
          <div role="radiogroup" aria-labelledby="appearance" className="mt-2.5 grid grid-cols-3 gap-1 rounded-[10px] border border-line bg-field p-1">
            {THEMES.map((option) => {
              const selected = props.theme === option.value
              return (
                <button
                  key={option.value}
                  type="button"
                  role="radio"
                  aria-checked={selected}
                  onClick={() => props.onThemeChange(option.value)}
                  className={
                    'flex h-8 items-center justify-center gap-1.5 rounded-md text-[13px] font-medium transition ' +
                    (selected ? 'bg-accent text-accent-ink shadow-card' : 'text-muted hover:text-ink')
                  }
                >
                  <ThemeIcon theme={option.value} />
                  {option.label}
                </button>
              )
            })}
          </div>
        </section>

        <section aria-labelledby="working-drive" className="rounded-xl border border-line bg-surface px-4 py-3.5 shadow-card">
          <h2 id="working-drive" className="text-[14px] font-semibold">
            Working drive
          </h2>
          <p className="mt-1 text-[13px] text-muted">
            Where projects are downloaded and built, and where the build history is kept. A build can need several
            gigabytes, so pick a drive with plenty of free space. Finished builds still go to the folder you choose on the
            main screen.
          </p>

          <div role="radiogroup" aria-labelledby="working-drive" className="mt-3 flex flex-col gap-2">
            <Option
              selected={storage.folder === ''}
              busy={busy === ''}
              onSelect={() => use('')}
              title="Automatic"
              subtitle={`Builds in ${storage.autoWorkspaceDir}`}
              free={autoFree}
              note={storage.autoDataFreeBytes < FULL_BYTES ? `History is kept on ${storage.autoDataDir.slice(0, 2)}, which is full` : undefined}
            />
            {storage.drives.map((drive) => (
              <Option
                key={drive.root}
                selected={samePath(drive.folder, storage.folder)}
                busy={busy === drive.folder}
                onSelect={() => use(drive.folder)}
                title={`Drive ${drive.root.slice(0, 2)}`}
                subtitle={drive.folder}
                free={drive.freeBytes}
                total={drive.totalBytes}
              />
            ))}
            {otherFolder && (
              <Option selected busy={false} onSelect={() => {}} title="Other folder" subtitle={storage.folder} free={storage.workspaceFreeBytes} />
            )}
          </div>

          <div className="mt-3 flex items-center justify-between gap-3">
            <span className="text-[12.5px] text-faint">Network and removable drives are not listed.</span>
            <button type="button" onClick={chooseOther} className={fieldButton}>
              <FolderIcon />
              Choose a folder
            </button>
          </div>

          {error && (
            <div className="mt-3">
              <Notice tone="bad">{error}</Notice>
            </div>
          )}
        </section>

        <section aria-labelledby="saved-packages" className="rounded-xl border border-line bg-surface px-4 py-3.5 shadow-card">
          <div className="flex items-center justify-between gap-3">
            <h2 id="saved-packages" className="text-[14px] font-semibold">
              Saved packages
            </h2>
            <button type="button" onClick={clearPackages} disabled={storage.savedPackages === 0 || clearing} className={fieldButton + ' disabled:cursor-not-allowed disabled:opacity-40'}>
              {clearing && <Spinner size={12} />}
              Clear
            </button>
          </div>
          <p className="mt-1 text-[13px] text-muted">
            {storage.savedPackages === 0
              ? 'After a website project is built, its downloaded packages are kept on the working drive so the next build of that project is much faster.'
              : `Packages are kept for ${storage.savedPackages} project${storage.savedPackages === 1 ? '' : 's'}, so their next builds are much faster. They can take over 1 GB per project. Clearing them is safe; the next build downloads them again.`}
          </p>
          <div className="mt-3 flex items-center justify-between gap-3">
            <span id="keep-for" className="text-[13px] text-muted">
              Keep after a project's last build
            </span>
            <div role="radiogroup" aria-labelledby="keep-for" className="grid grid-cols-4 gap-1 rounded-[10px] border border-line bg-field p-1">
              {KEEP_OPTIONS.map((option) => {
                const selected = storage.packageKeepDays === option.days
                return (
                  <button
                    key={option.days}
                    type="button"
                    role="radio"
                    aria-checked={selected}
                    onClick={() => keepFor(option.days)}
                    className={
                      'h-7 rounded-md px-2.5 text-[12.5px] font-medium whitespace-nowrap transition ' +
                      (selected ? 'bg-accent text-accent-ink shadow-card' : 'text-muted hover:text-ink')
                    }
                  >
                    {option.label}
                  </button>
                )
              })}
            </div>
          </div>
        </section>

        <section className="rounded-xl border border-line bg-surface px-4 py-3.5 text-[13px] shadow-card">
          <h2 className="text-[14px] font-semibold">In use now</h2>
          <dl className="mt-2 flex flex-col gap-2">
            <Location label="Downloads and builds" path={storage.workspaceDir} free={storage.workspaceFreeBytes} />
            <Location label="Build history and logs" path={storage.dataDir} free={storage.dataFreeBytes} />
          </dl>
        </section>
      </main>

      <footer className="flex flex-col gap-3 border-t border-line bg-surface px-5 py-4">
        <button type="button" onClick={props.onClose} className={primaryButton + ' w-full'}>
          Done
        </button>
      </footer>
    </div>
  )
}

function Option(props: {
  selected: boolean
  busy: boolean
  onSelect: () => void
  title: string
  subtitle: string
  free: number
  total?: number
  note?: string
}) {
  const low = props.free < LOW_SPACE_BYTES
  const used = props.total ? Math.min(100, Math.max(0, ((props.total - props.free) / props.total) * 100)) : null

  return (
    <button
      type="button"
      role="radio"
      aria-checked={props.selected}
      onClick={props.onSelect}
      disabled={props.busy}
      className={
        'flex items-center gap-3 rounded-lg border px-3 py-2.5 text-left transition ' +
        (props.selected ? 'border-accent bg-accent-soft' : 'border-line bg-field hover:border-line-strong')
      }
    >
      <span
        className={
          'grid size-[18px] shrink-0 place-items-center rounded-full border ' +
          (props.selected ? 'border-accent bg-accent text-accent-ink' : 'border-line-strong')
        }
      >
        {props.busy ? <Spinner size={10} /> : props.selected && <CheckIcon size={11} />}
      </span>
      <span className="min-w-0 flex-1">
        <span className="block font-medium">{props.title}</span>
        <span className="block truncate font-mono text-[12px] text-muted">{props.subtitle}</span>
        {props.note && <span className="mt-0.5 block text-[12.5px] text-warn">{props.note}</span>}
      </span>
      <span className="w-28 shrink-0 text-right">
        <span className={'block text-[13px] font-medium tabular-nums ' + (low ? 'text-warn' : '')}>
          {low ? 'Almost full' : `${formatBytes(props.free)} free`}
        </span>
        {used !== null && (
          <span className="mt-1.5 block h-1 overflow-hidden rounded-full bg-line">
            <span className={'block h-full rounded-full ' + (low ? 'bg-warn' : 'bg-accent')} style={{ width: `${used}%` }} />
          </span>
        )}
        {low && used === null && <span className="block text-[12px] text-faint tabular-nums">{formatBytes(props.free)} free</span>}
      </span>
    </button>
  )
}

function ThemeIcon(props: { theme: Theme }) {
  const common = { width: 14, height: 14, viewBox: '0 0 16 16', fill: 'none', stroke: 'currentColor', strokeWidth: 1.5, strokeLinecap: 'round', strokeLinejoin: 'round' } as const
  switch (props.theme) {
    case 'light':
      return (
        <svg {...common} aria-hidden="true">
          <circle cx="8" cy="8" r="2.8" />
          <path d="M8 1.5v1.6M8 12.9v1.6M1.5 8h1.6M12.9 8h1.6M3.4 3.4l1.1 1.1M11.5 11.5l1.1 1.1M12.6 3.4l-1.1 1.1M4.5 11.5l-1.1 1.1" />
        </svg>
      )
    case 'dark':
      return (
        <svg {...common} aria-hidden="true">
          <path d="M13.5 9.6A5.8 5.8 0 0 1 6.4 2.5a5.8 5.8 0 1 0 7.1 7.1z" />
        </svg>
      )
    default:
      return (
        <svg {...common} aria-hidden="true">
          <rect x="1.8" y="2.8" width="12.4" height="8.4" rx="1.4" />
          <path d="M5.5 13.6h5M8 11.2v2.4" />
        </svg>
      )
  }
}

function Location(props: { label: string; path: string; free: number }): ReactNode {
  return (
    <div className="flex items-baseline justify-between gap-3">
      <div className="min-w-0">
        <dt className="text-faint">{props.label}</dt>
        <dd className="selectable truncate font-mono text-[12px]">{props.path}</dd>
      </div>
      <span className={'shrink-0 tabular-nums ' + (props.free < LOW_SPACE_BYTES ? 'text-warn' : 'text-muted')}>{formatBytes(props.free)} free</span>
    </div>
  )
}
