import { useEffect, useRef, useState } from 'react'
import type { KeyboardEvent, ReactNode } from 'react'
import { ClipboardSetText } from '../wailsjs/runtime/runtime'

// Small building blocks shared by the three screens of the app.

export const primaryButton =
  'inline-flex h-11 items-center justify-center gap-2 rounded-[10px] bg-accent px-5 text-[14px] font-semibold text-accent-ink shadow-card transition hover:bg-accent-hover active:translate-y-px disabled:cursor-not-allowed disabled:opacity-40'
export const secondaryButton =
  'inline-flex h-11 items-center justify-center gap-2 rounded-[10px] border border-line-strong bg-surface px-5 text-[14px] font-medium transition hover:bg-field active:translate-y-px disabled:cursor-not-allowed disabled:opacity-40'
export const fieldButton =
  'inline-flex h-9 shrink-0 items-center justify-center gap-2 rounded-lg border border-line-strong bg-surface px-3.5 text-[13.5px] font-medium transition hover:bg-field active:translate-y-px'
export const quietButton =
  'inline-flex items-center gap-1.5 rounded-md text-[13px] font-medium text-accent transition hover:text-accent-hover disabled:cursor-not-allowed disabled:text-faint'
const inputBase =
  'h-9 w-full min-w-0 rounded-lg border border-line bg-field px-3 text-ink transition placeholder:text-faint hover:border-line-strong focus:border-accent disabled:bg-canvas disabled:text-faint disabled:hover:border-line'

export function Section(props: { step: number; title: string; action?: ReactNode; children: ReactNode }) {
  const titleId = `section-${props.step}`
  return (
    <section aria-labelledby={titleId} className="rounded-xl border border-line bg-surface px-4 py-3.5 shadow-card">
      <div className="mb-2.5 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2.5">
          <span className="grid size-5 place-items-center rounded-full bg-accent-soft text-[11px] font-semibold text-accent tabular-nums">
            {props.step}
          </span>
          <h2 id={titleId} className="text-[14px] font-semibold">
            {props.title}
          </h2>
        </div>
        {props.action}
      </div>
      {props.children}
    </section>
  )
}

export function Field(props: { label: string; htmlFor: string; hint?: string; children: ReactNode }) {
  return (
    <div className="min-w-0">
      <label htmlFor={props.htmlFor} className="mb-1.5 block text-[12.5px] font-medium text-muted">
        {props.label}
      </label>
      {props.children}
      {props.hint && <p className="mt-1.5 text-[12.5px] text-faint">{props.hint}</p>}
    </div>
  )
}

interface TextInputProps {
  id: string
  value: string
  onChange: (value: string) => void
  placeholder?: string
  autoFocus?: boolean
  inputMode?: 'numeric'
  label?: string
  disabled?: boolean
  mono?: boolean
  // trailing leaves room on the right for a button placed over the box.
  trailing?: boolean
}

export function TextInput(props: TextInputProps) {
  return (
    <input
      id={props.id}
      type="text"
      value={props.value}
      onChange={(e) => props.onChange(e.target.value)}
      placeholder={props.placeholder}
      autoFocus={props.autoFocus}
      autoComplete="off"
      inputMode={props.inputMode}
      aria-label={props.label}
      disabled={props.disabled}
      spellCheck={false}
      className={inputBase + (props.mono ? ' font-mono text-[13px]' : '') + (props.trailing ? ' pr-9' : '')}
    />
  )
}

export function PasswordInput(props: { id: string; value: string; onChange: (value: string) => void; label?: string }) {
  const [shown, setShown] = useState(false)
  return (
    <div className="relative">
      <input
        id={props.id}
        type={shown ? 'text' : 'password'}
        value={props.value}
        onChange={(e) => props.onChange(e.target.value)}
        autoComplete="off"
        aria-label={props.label}
        spellCheck={false}
        className={inputBase + ' pr-9'}
      />
      <button
        type="button"
        onClick={() => setShown(!shown)}
        aria-label={shown ? 'Hide password' : 'Show password'}
        aria-pressed={shown}
        title={shown ? 'Hide password' : 'Show password'}
        className="absolute top-0.5 right-1 grid size-8 place-items-center rounded-md text-faint transition hover:text-ink"
      >
        <EyeIcon off={shown} />
      </button>
    </div>
  )
}

export interface PickerItem {
  key: string
  label: string
  hint?: string
  selected?: boolean
}

// Picker is a searchable drop-down list. Place it inside a `relative` element; it opens below it.
// With `multi`, picking toggles an item and the list stays open.
export function Picker(props: {
  items: PickerItem[]
  onPick: (key: string) => void
  onClose: () => void
  searchPlaceholder: string
  emptyText: string
  multi?: boolean
  onRemove?: (key: string) => void
  footer?: ReactNode
}) {
  const [query, setQuery] = useState('')
  const [active, setActive] = useState(0)
  const panel = useRef<HTMLDivElement>(null)
  const list = useRef<HTMLUListElement>(null)

  const words = query.toLowerCase().split(/\s+/).filter(Boolean)
  const shown = props.items.filter((item) => {
    const text = (item.label + ' ' + (item.hint ?? '')).toLowerCase()
    return words.every((word) => text.includes(word))
  })
  const activeIndex = Math.min(active, Math.max(0, shown.length - 1))

  useEffect(() => {
    function onPointerDown(e: PointerEvent) {
      if (panel.current && !panel.current.contains(e.target as Node)) props.onClose()
    }
    document.addEventListener('pointerdown', onPointerDown)
    return () => document.removeEventListener('pointerdown', onPointerDown)
  }, [props.onClose])

  useEffect(() => {
    list.current?.children[activeIndex]?.scrollIntoView({ block: 'nearest' })
  }, [activeIndex])

  function pick(key: string) {
    props.onPick(key)
    if (!props.multi) props.onClose()
  }

  function onKeyDown(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setActive(Math.min(activeIndex + 1, shown.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setActive(Math.max(activeIndex - 1, 0))
    } else if (e.key === 'Enter') {
      e.preventDefault()
      if (shown[activeIndex]) pick(shown[activeIndex].key)
    } else if (e.key === 'Escape') {
      e.preventDefault()
      props.onClose()
    }
  }

  return (
    <div
      ref={panel}
      className="absolute top-full right-0 left-0 z-20 mt-1.5 overflow-hidden rounded-[10px] border border-line-strong bg-surface shadow-[0_12px_32px_rgb(0_0_0/0.18)]"
    >
      <div className="flex items-center gap-2 border-b border-line px-3 text-faint">
        <SearchIcon />
        <input
          type="text"
          value={query}
          onChange={(e) => {
            setQuery(e.target.value)
            setActive(0)
          }}
          onKeyDown={onKeyDown}
          placeholder={props.searchPlaceholder}
          aria-label={props.searchPlaceholder}
          autoFocus
          autoComplete="off"
          spellCheck={false}
          className="h-9 w-full min-w-0 bg-transparent text-ink placeholder:text-faint"
        />
      </div>

      {shown.length === 0 ? (
        <p className="px-3 py-3 text-[13px] text-muted">{props.items.length === 0 ? props.emptyText : 'Nothing matches your search.'}</p>
      ) : (
        <ul ref={list} role="listbox" aria-multiselectable={props.multi} className="max-h-52 overflow-y-auto p-1">
          {shown.map((item, i) => (
            <li
              key={item.key}
              role="option"
              aria-selected={!!item.selected}
              onClick={() => pick(item.key)}
              onPointerMove={() => setActive(i)}
              className={'flex h-8 cursor-pointer items-center gap-2 rounded-md px-2 text-[13px] ' + (i === activeIndex ? 'bg-accent-soft' : '')}
            >
              <span className={'grid w-4 shrink-0 place-items-center text-accent ' + (item.selected ? '' : 'invisible')}>
                <CheckIcon size={12} />
              </span>
              <span className="min-w-0 flex-1 truncate font-medium">{item.label}</span>
              {item.hint && <span className="shrink-0 text-faint">{item.hint}</span>}
              {props.onRemove && (
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation()
                    props.onRemove?.(item.key)
                  }}
                  aria-label={`Forget ${item.label}`}
                  title="Forget this address"
                  className="grid size-6 shrink-0 place-items-center rounded text-faint transition hover:text-ink"
                >
                  <CrossIcon size={11} />
                </button>
              )}
            </li>
          ))}
        </ul>
      )}

      {props.footer && <div className="border-t border-line px-3 py-2 text-[12.5px] text-faint">{props.footer}</div>}
    </div>
  )
}

export function Toggle(props: { checked: boolean; onChange: (checked: boolean) => void; children: ReactNode }) {
  return (
    <label className="flex w-fit cursor-pointer items-center gap-2.5 text-[13px]">
      <input type="checkbox" checked={props.checked} onChange={(e) => props.onChange(e.target.checked)} className="peer sr-only" />
      <span className="relative h-5 w-9 shrink-0 rounded-full bg-line-strong transition peer-checked:bg-accent peer-focus-visible:ring-2 peer-focus-visible:ring-accent peer-focus-visible:ring-offset-2 peer-focus-visible:ring-offset-surface after:absolute after:top-0.5 after:left-0.5 after:size-4 after:rounded-full after:bg-white after:shadow-sm after:transition-transform peer-checked:after:translate-x-4" />
      <span>{props.children}</span>
    </label>
  )
}

export function Notice(props: { tone: 'bad' | 'warn'; children: ReactNode }) {
  const tone = props.tone === 'bad' ? 'bg-bad-soft text-bad' : 'bg-warn-soft text-warn'
  return (
    <div role="alert" className={'flex items-start gap-2.5 rounded-lg px-3 py-2.5 text-[13px] ' + tone}>
      <span className="mt-px shrink-0">
        <AlertIcon />
      </span>
      <div className="min-w-0">{props.children}</div>
    </div>
  )
}

export function CopyButton(props: { text: string; label?: string }) {
  const [copied, setCopied] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  useEffect(() => () => clearTimeout(timer.current), [])

  async function copy() {
    await ClipboardSetText(props.text)
    setCopied(true)
    clearTimeout(timer.current)
    timer.current = setTimeout(() => setCopied(false), 1600)
  }

  return (
    <button type="button" onClick={copy} className={quietButton + ' shrink-0'}>
      {copied ? <CheckIcon size={13} /> : <CopyIcon />}
      {copied ? 'Copied' : (props.label ?? 'Copy')}
    </button>
  )
}

export function LogDetails(props: { lines: string[] }) {
  const [open, setOpen] = useState(false)
  const box = useRef<HTMLPreElement>(null)

  useEffect(() => {
    if (open && box.current) box.current.scrollTop = box.current.scrollHeight
  }, [open, props.lines])

  return (
    <div>
      <button type="button" onClick={() => setOpen(!open)} aria-expanded={open} className="inline-flex items-center gap-1.5 text-[13px] text-muted transition hover:text-ink">
        <span className={'transition-transform ' + (open ? 'rotate-90' : '')}>
          <ChevronIcon />
        </span>
        {open ? 'Hide technical details' : 'Show technical details'}
      </button>
      {open && (
        <pre
          ref={box}
          className="mt-2 h-56 overflow-auto rounded-lg border border-line bg-surface p-3 font-mono text-[12px] leading-relaxed whitespace-pre-wrap text-muted"
        >
          {props.lines.join('\n')}
        </pre>
      )}
    </div>
  )
}

export function Elapsed(props: { since: number }) {
  const [now, setNow] = useState(Date.now())
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [])

  const seconds = Math.max(0, Math.floor((now - props.since) / 1000))
  return (
    <span className="tabular-nums">
      {Math.floor(seconds / 60)}:{String(seconds % 60).padStart(2, '0')}
    </span>
  )
}

export function Spinner(props: { size?: number }) {
  const size = props.size ?? 16
  return (
    <span
      aria-hidden="true"
      style={{ width: size, height: size }}
      className="inline-block shrink-0 animate-spin rounded-full border-2 border-current border-t-transparent opacity-80"
    />
  )
}

const stroke = { fill: 'none', stroke: 'currentColor', strokeLinecap: 'round', strokeLinejoin: 'round' } as const

export function CheckIcon(props: { size: number }) {
  return (
    <svg width={props.size} height={props.size} viewBox="0 0 16 16" {...stroke} strokeWidth="2.2" aria-hidden="true">
      <path d="M3 8.5l3.2 3.2L13 5" />
    </svg>
  )
}

export function CrossIcon(props: { size: number }) {
  return (
    <svg width={props.size} height={props.size} viewBox="0 0 16 16" {...stroke} strokeWidth="2" aria-hidden="true">
      <path d="M4 4l8 8M12 4l-8 8" />
    </svg>
  )
}

export function PlusIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 16 16" {...stroke} strokeWidth="2" aria-hidden="true">
      <path d="M8 3v10M3 8h10" />
    </svg>
  )
}

export function SearchIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 16 16" {...stroke} strokeWidth="1.8" aria-hidden="true">
      <circle cx="7" cy="7" r="4.5" />
      <path d="M10.5 10.5L14 14" />
    </svg>
  )
}

export function FolderIcon() {
  return (
    <svg width="15" height="15" viewBox="0 0 16 16" {...stroke} strokeWidth="1.5" aria-hidden="true">
      <path d="M1.8 4.2c0-.7.5-1.2 1.2-1.2h3l1.5 1.6H13c.7 0 1.2.5 1.2 1.2v6c0 .7-.5 1.2-1.2 1.2H3c-.7 0-1.2-.5-1.2-1.2V4.2z" />
    </svg>
  )
}

export function PlayIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
      <path d="M4.5 2.7c0-.6.7-1 1.2-.7l8 5.3c.5.3.5 1 0 1.4l-8 5.3c-.5.3-1.2 0-1.2-.7V2.7z" />
    </svg>
  )
}

export function StopIcon() {
  return (
    <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
      <rect x="3" y="3" width="10" height="10" rx="2" />
    </svg>
  )
}

export function GearIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" {...stroke} strokeWidth="1.8" aria-hidden="true">
      <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z" />
      <circle cx="12" cy="12" r="3" />
    </svg>
  )
}

export function BackIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" {...stroke} strokeWidth="1.8" aria-hidden="true">
      <path d="M9.5 3.5L5 8l4.5 4.5" />
    </svg>
  )
}

function CopyIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 16 16" {...stroke} strokeWidth="1.5" aria-hidden="true">
      <rect x="5.5" y="5.5" width="8" height="8" rx="1.5" />
      <path d="M10.5 5.5v-2c0-.6-.4-1-1-1h-6c-.6 0-1 .4-1 1v6c0 .6.4 1 1 1h2" />
    </svg>
  )
}

export function ChevronDownIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 16 16" {...stroke} strokeWidth="1.8" aria-hidden="true">
      <path d="M3.5 6L8 10.5 12.5 6" />
    </svg>
  )
}

function ChevronIcon() {
  return (
    <svg width="12" height="12" viewBox="0 0 16 16" {...stroke} strokeWidth="2" aria-hidden="true">
      <path d="M6 3.5L10.5 8 6 12.5" />
    </svg>
  )
}

function AlertIcon() {
  return (
    <svg width="15" height="15" viewBox="0 0 16 16" {...stroke} strokeWidth="1.6" aria-hidden="true">
      <circle cx="8" cy="8" r="6.2" />
      <path d="M8 4.8v3.6M8 11v.2" />
    </svg>
  )
}

function EyeIcon(props: { off: boolean }) {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" {...stroke} strokeWidth="1.4" aria-hidden="true">
      <path d="M1.5 8S4 3.5 8 3.5 14.5 8 14.5 8 12 12.5 8 12.5 1.5 8 1.5 8z" />
      <circle cx="8" cy="8" r="2" />
      {props.off && <path d="M2.5 13.5l11-11" />}
    </svg>
  )
}

export function BoxIcon(props: { size: number }) {
  return (
    <svg width={props.size} height={props.size} viewBox="0 0 16 16" {...stroke} strokeWidth="1.5" aria-hidden="true">
      <path d="M8 1.8l5.5 3v6.4L8 14.2l-5.5-3V4.8L8 1.8z" />
      <path d="M2.5 4.8L8 7.8l5.5-3M8 7.8v6.4" />
    </svg>
  )
}
