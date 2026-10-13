import { useCallback, useEffect, useRef, useState } from 'react'
import type { FormEvent, ReactNode } from 'react'
import { CancelBuild, CheckTools, ChooseFolder, FetchFolders, ForgetRepository, GetStorage, LoadSettings, OpenFolder, SetTheme, StartBuild } from '../wailsjs/go/main/App'
import { main } from '../wailsjs/go/models'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { Settings } from './Settings'
import { applyTheme, rememberedTheme, toTheme } from './theme'
import type { Theme } from './theme'
import {
  BoxIcon,
  CheckIcon,
  ChevronDownIcon,
  CopyButton,
  CrossIcon,
  Elapsed,
  Field,
  FolderIcon,
  GearIcon,
  LogDetails,
  Notice,
  PasswordInput,
  Picker,
  PlayIcon,
  PlusIcon,
  SearchIcon,
  Section,
  Spinner,
  StopIcon,
  TextInput,
  Toggle,
  fieldButton,
  primaryButton,
  quietButton,
  secondaryButton,
} from './ui'

type View = 'form' | 'settings' | 'running' | 'done'
type StepState = 'waiting' | 'active' | 'done' | 'failed'

// Event payloads sent by desktop/app.go (StageUpdate and BuildOutcome).
interface StageUpdate {
  stage: string
  detail: string
}

interface BuildOutcome {
  status: 'success' | 'failed' | 'cancelled' | 'nothing'
  buildNumber: number
  buildName: string
  buildDir: string
  revision: string
  failedStage: string
  message: string
  details: string
  projects: { name: string; type: string; success: boolean }[]
}

// One row of the "Folders to build" list.
interface ProjectRow {
  folder: string
  script: string
}

const STEPS = [
  { key: 'tools', label: 'Checking this PC' },
  { key: 'download', label: 'Downloading the project from SVN' },
  { key: 'build', label: 'Building' },
  { key: 'save', label: 'Saving to the shared folder' },
]

const PROJECT_KINDS: Record<string, string> = {
  react: 'Website (React)',
  next: 'Website (Next.js)',
  dotnet: '.NET application',
}

const SHORT_KINDS: Record<string, string> = {
  react: 'Website',
  next: 'Website',
  dotnet: '.NET',
}

const TOOL_LABELS = [
  { keys: ['svn'], label: 'SVN', missing: 'Subversion (SVN) is not installed on this PC' },
  { keys: ['node', 'npm'], label: 'Node', missing: 'Node.js is not installed, so website projects cannot be built on this PC' },
  { keys: ['dotnet'], label: '.NET', missing: 'The .NET SDK is not installed, so .NET projects cannot be built on this PC' },
]

const MAX_LOG_LINES = 3000
const DEFAULT_SCRIPT = 'build'
const EMPTY_ROW: ProjectRow = { folder: '', script: '' }

// Folder names are compared the way the build engine does: ignoring case, slash style and outer slashes.
function sameFolder(a: string, b: string) {
  const normalize = (s: string) => s.trim().replace(/\\/g, '/').replace(/^\/+|\/+$/g, '').toLowerCase()
  return normalize(a) === normalize(b)
}

const sameAddress = (a: string, b: string) => a.trim().replace(/\/+$/, '').toLowerCase() === b.trim().replace(/\/+$/, '').toLowerCase()

// describeAddress splits an SVN address for the drop-down: the path is what tells addresses apart,
// the server is secondary.
function describeAddress(url: string) {
  const match = url.match(/^[a-z+]+:\/\/+([^/]+)\/*(.*)$/i)
  if (!match || !match[2]) return { label: url }
  return { label: match[2], hint: match[1] }
}

const isNetworkFolder = (path: string) => /^\s*[\\/]{2}[^\\/]+[\\/]+[^\\/]/.test(path)

export default function App() {
  const [view, setView] = useState<View>('form')
  const [repositoryUrl, setRepositoryUrl] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [outputDir, setOutputDir] = useState('')
  const [revision, setRevision] = useState('')
  const [projects, setProjects] = useState<ProjectRow[]>([EMPTY_ROW])
  const [cleanBuild, setCleanBuild] = useState(false)
  const [recentRepos, setRecentRepos] = useState<string[]>([])
  // Which drop-down list is open, if any.
  const [picker, setPicker] = useState<'repos' | 'folders' | null>(null)
  const closePicker = useCallback(() => setPicker(null), [])
  const [shareSignIn, setShareSignIn] = useState(false)
  const [shareUsername, setShareUsername] = useState('')
  const [sharePassword, setSharePassword] = useState('')
  const [formError, setFormError] = useState('')
  const [starting, setStarting] = useState(false)
  const [found, setFound] = useState<main.RemoteFolder[] | null>(null)
  const [fetching, setFetching] = useState(false)
  const [fetchError, setFetchError] = useState('')
  const [tools, setTools] = useState<main.ToolStatus[]>([])
  const [storage, setStorage] = useState<main.StorageInfo | null>(null)
  const [theme, setTheme] = useState<Theme>(rememberedTheme)

  const [stage, setStage] = useState('')
  const [detail, setDetail] = useState('')
  const [logLines, setLogLines] = useState<string[]>([])
  const [outcome, setOutcome] = useState<BuildOutcome | null>(null)
  const [startedAt, setStartedAt] = useState(0)
  const [stopping, setStopping] = useState(false)
  const [openError, setOpenError] = useState('')

  useEffect(() => {
    LoadSettings().then((s) => {
      setRepositoryUrl(s.repositoryUrl ?? '')
      setUsername(s.username ?? '')
      setOutputDir(s.outputDir ?? '')
      setShareUsername(s.shareUsername ?? '')
      // Settings saved before the list existed only know the last address.
      setRecentRepos(s.recentRepositories?.length ? s.recentRepositories : s.repositoryUrl ? [s.repositoryUrl] : [])
      setShareSignIn(!!s.shareUsername)
      setTheme(toTheme(s.theme))
      applyTheme(toTheme(s.theme))
      if (s.projects?.length) setProjects(s.projects.map((p) => ({ folder: p.folder ?? '', script: p.script ?? '' })))
    })
    CheckTools().then(setTools)
    GetStorage().then(setStorage)

    const offStage = EventsOn('build:stage', (update: StageUpdate) => {
      setStage(update.stage)
      setDetail(update.detail)
    })
    const offLog = EventsOn('build:log', (line: string) => {
      setLogLines((lines) => (lines.length >= MAX_LOG_LINES ? [...lines.slice(-MAX_LOG_LINES + 1), line] : [...lines, line]))
    })
    const offDone = EventsOn('build:done', (result: BuildOutcome) => {
      setOutcome(result)
      setView('done')
      // A build changes how much room is left on the working drive.
      GetStorage().then(setStorage)
    })
    return () => {
      offStage()
      offLog()
      offDone()
    }
  }, [])

  // The sign-in boxes open at the bottom of a scrolling form; bring them into view when the user
  // turns them on or the server asks for an account. Not when saved settings switch them on at
  // start, which would open the app scrolled to the bottom.
  const signInBox = useRef<HTMLDivElement>(null)
  const revealSignIn = useRef(false)
  useEffect(() => {
    if (shareSignIn && revealSignIn.current) signInBox.current?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
    revealSignIn.current = false
  }, [shareSignIn])

  function showSignIn(on: boolean) {
    revealSignIn.current = on
    setShareSignIn(on)
  }

  const missing = (key: string) => tools.some((t) => t.key === key && !t.installed)
  const svnMissing = missing('svn')
  const networkFolder = isNetworkFolder(outputDir)
  const useShareSignIn = networkFolder && shareSignIn

  async function startBuild(e: FormEvent) {
    e.preventDefault()
    setFormError('')
    setStarting(true)
    // Reset before starting: progress events can arrive before StartBuild resolves.
    setStage('tools')
    setDetail('')
    setLogLines([])
    setOutcome(null)
    setStopping(false)
    setOpenError('')
    try {
      const result = await StartBuild(
        main.BuildRequest.createFrom({
          repositoryUrl,
          username,
          password,
          outputDir,
          revision,
          projects,
          cleanBuild,
          shareUsername: useShareSignIn ? shareUsername : '',
          sharePassword: useShareSignIn ? sharePassword : '',
        }),
      )
      if (result.started) {
        rememberAddress(repositoryUrl)
        // A clean build is a one-off; the next build goes back to reusing packages.
        setCleanBuild(false)
        setStartedAt(Date.now())
        setView('running')
      } else {
        if (result.needsSignIn) showSignIn(true)
        setFormError(result.message)
      }
    } catch (err) {
      setFormError(String(err))
    } finally {
      setStarting(false)
    }
  }

  // Mirrors what the backend saves, so the address list is current without reloading settings.
  function rememberAddress(url: string) {
    const address = url.trim().replace(/\/+$/, '')
    if (address) setRecentRepos((list) => [address, ...list.filter((other) => !sameAddress(other, address))].slice(0, 20))
  }

  function updateProject(index: number, change: Partial<ProjectRow>) {
    setProjects((rows) => rows.map((row, i) => (i === index ? { ...row, ...change } : row)))
  }

  function addProject() {
    setProjects((rows) => [...rows, EMPTY_ROW])
  }

  function removeProject(index: number) {
    setProjects((rows) => (rows.length === 1 ? [EMPTY_ROW] : rows.filter((_, i) => i !== index)))
  }

  function changeRepositoryUrl(url: string) {
    setRepositoryUrl(url)
    // Folders found earlier belong to the previous address.
    setFound(null)
    setFetchError('')
  }

  async function fetchFolders() {
    setFetchError('')
    setFetching(true)
    try {
      const folders = await FetchFolders(main.BuildRequest.createFrom({ repositoryUrl, username, password, revision }))
      setFound(folders)
      rememberAddress(repositoryUrl)
      // Show what was found straight away, ready to pick from.
      if (folders.length > 0) setPicker('folders')
    } catch (err) {
      setFound(null)
      setFetchError(String(err))
    } finally {
      setFetching(false)
    }
  }

  const isSelected = (folder: string) => projects.some((row) => sameFolder(row.folder, folder))
  const chosenCount = found ? found.filter((f) => isSelected(f.folder)).length : 0
  const isDotnet = (folder: string) => !!found?.some((f) => f.type === 'dotnet' && sameFolder(f.folder, folder))

  function toggleFolder(folder: string) {
    setProjects((rows) => {
      if (rows.some((row) => sameFolder(row.folder, folder))) {
        const rest = rows.filter((row) => !sameFolder(row.folder, folder))
        return rest.length ? rest : [EMPTY_ROW]
      }
      // Fill the blank starter row before adding new ones.
      const filled = rows.filter((row) => row.folder.trim() !== '' || row.script.trim() !== '')
      return [...filled, { folder, script: '' }]
    })
  }

  async function browse() {
    const chosen = await ChooseFolder(outputDir, 'Choose where builds are saved')
    if (chosen) setOutputDir(chosen)
  }

  function changeTheme(next: Theme) {
    setTheme(next)
    applyTheme(next)
    // The choice already applies; a failed save only means it is not remembered.
    SetTheme(next).catch(() => {})
  }

  function openSettings() {
    // Free space changes while the app is open; show current numbers.
    GetStorage().then(setStorage)
    setView('settings')
  }

  function stopBuild() {
    setStopping(true)
    CancelBuild()
  }

  if (view === 'settings' && storage) {
    return <Settings storage={storage} onChange={setStorage} theme={theme} onThemeChange={changeTheme} onClose={() => setView('form')} />
  }

  if (view === 'form' || view === 'settings') {
    return (
      <form onSubmit={startBuild} className="flex h-full flex-col">
        <Body>
          <header className="flex items-center justify-between gap-4 px-1 pb-1">
            <div className="flex items-center gap-3">
              <span className="grid size-10 place-items-center rounded-[10px] bg-accent text-accent-ink shadow-card">
                <BoxIcon size={20} />
              </span>
              <div>
                <h1 className="font-display text-[18px] leading-tight font-semibold">Build a project</h1>
                <p className="text-[13px] text-muted">From SVN to your shared folder</p>
              </div>
            </div>
            <div className="flex items-center gap-1.5">
              <ul aria-label="Build tools on this PC" className="flex items-center gap-1.5">
                {tools.length > 0 && TOOL_LABELS.map((tool) => {
                  const isMissing = tool.keys.some(missing)
                  return (
                    <li
                      key={tool.label}
                      title={isMissing ? tool.missing : `${tool.label} is installed`}
                      className="flex h-6 items-center gap-1.5 rounded-full border border-line bg-surface px-2 text-[11.5px] font-medium text-muted"
                    >
                      <span className={'size-1.5 rounded-full ' + (isMissing ? 'bg-warn' : 'bg-good')} />
                      {tool.label}
                      <span className="sr-only">{isMissing ? 'missing' : 'installed'}</span>
                    </li>
                  )
                })}
              </ul>
              <button
                type="button"
                onClick={openSettings}
                aria-label="Settings"
                title="Settings"
                className="relative grid size-8 place-items-center rounded-lg text-muted transition hover:bg-surface hover:text-ink"
              >
                <GearIcon />
                {storage?.warning && <span className="absolute top-1 right-1 size-2 rounded-full border-2 border-canvas bg-warn" />}
              </button>
            </div>
          </header>

          {storage?.warning && (
            <Notice tone="warn">
              {storage.warning}{' '}
              <button type="button" onClick={openSettings} className="font-semibold underline underline-offset-2">
                Choose another drive
              </button>
            </Notice>
          )}

          {svnMissing && (
            <Notice tone="bad">
              Subversion (SVN) is not installed on this PC, so builds can't start. Ask IT to install the SVN command-line
              tools, then reopen this app.
            </Notice>
          )}

          <Section step={1} title="Source">
            <div className="flex flex-col gap-3">
              <Field label="Project address in SVN" htmlFor="repo">
                <div className="relative">
                  <TextInput
                    id="repo"
                    value={repositoryUrl}
                    onChange={changeRepositoryUrl}
                    placeholder="https://svn.company.com/repos/MyProject/trunk"
                    autoFocus
                    trailing
                  />
                  <button
                    type="button"
                    onClick={() => setPicker(picker === 'repos' ? null : 'repos')}
                    aria-label="Addresses used before"
                    aria-haspopup="listbox"
                    aria-expanded={picker === 'repos'}
                    title="Addresses used before"
                    className="absolute top-0.5 right-1 grid size-8 place-items-center rounded-md text-faint transition hover:text-ink"
                  >
                    <ChevronDownIcon />
                  </button>
                  {picker === 'repos' && (
                    <Picker
                      items={recentRepos.map((url) => ({ key: url, ...describeAddress(url), selected: sameAddress(url, repositoryUrl) }))}
                      onPick={changeRepositoryUrl}
                      onClose={closePicker}
                      onRemove={(url) => ForgetRepository(url).then(setRecentRepos)}
                      searchPlaceholder="Search addresses"
                      emptyText="Addresses you build from are remembered here."
                      footer="Addresses used on this PC. New ones are added when you build."
                    />
                  )}
                </div>
              </Field>
              <div className="grid grid-cols-[1fr_1fr_6rem] gap-3">
                <Field label="SVN user name" htmlFor="user">
                  <TextInput id="user" value={username} onChange={setUsername} />
                </Field>
                <Field label="SVN password" htmlFor="pass">
                  <PasswordInput id="pass" value={password} onChange={setPassword} />
                </Field>
                <Field label="Revision" htmlFor="rev">
                  <TextInput id="rev" value={revision} onChange={setRevision} placeholder="Latest" inputMode="numeric" />
                </Field>
              </div>
              <p className="-mt-1 text-[12.5px] text-faint">Passwords are used for this build only and are never saved.</p>
            </div>
          </Section>

          <Section
            step={2}
            title="Folders to build"
            action={
              <button type="button" onClick={fetchFolders} disabled={fetching || svnMissing} className={quietButton}>
                {fetching ? <Spinner size={12} /> : <SearchIcon />}
                {fetching ? 'Looking in SVN…' : found ? 'Look in SVN again' : 'Find folders in SVN'}
              </button>
            }
          >
            {fetchError && (
              <div className="mb-3">
                <Notice tone="bad">{fetchError}</Notice>
              </div>
            )}
            {found && found.length === 0 && (
              <p className="mb-3 text-[13px] text-muted">No React, Next.js or .NET project was found at this address.</p>
            )}
            {found && found.length > 0 && (
              <div className="relative mb-3">
                <button
                  type="button"
                  onClick={() => setPicker(picker === 'folders' ? null : 'folders')}
                  aria-haspopup="listbox"
                  aria-expanded={picker === 'folders'}
                  className="flex h-9 w-full items-center justify-between gap-2 rounded-lg border border-line bg-field px-3 text-left transition hover:border-line-strong"
                >
                  <span className="truncate">
                    Choose from {found.length} folder{found.length === 1 ? '' : 's'} found in SVN
                    {chosenCount > 0 && <span className="text-muted"> · {chosenCount} chosen</span>}
                  </span>
                  <span className="shrink-0 text-faint">
                    <ChevronDownIcon />
                  </span>
                </button>
                {picker === 'folders' && (
                  <Picker
                    multi
                    items={found.map((f) => ({
                      key: f.folder,
                      label: f.folder === '.' ? 'Top folder' : f.folder,
                      hint: SHORT_KINDS[f.type] ?? f.type,
                      selected: isSelected(f.folder),
                    }))}
                    onPick={toggleFolder}
                    onClose={closePicker}
                    searchPlaceholder="Search folders"
                    emptyText="No folders were found."
                    footer="Click a folder to add or remove it."
                  />
                )}
              </div>
            )}

            <div className="flex flex-col gap-2">
              <div className="grid grid-cols-[1fr_9rem_2.5rem] gap-2 text-[12.5px] font-medium text-muted">
                <span>Folder in the project</span>
                <span>Build script</span>
              </div>
              {projects.map((project, i) => (
                <div key={i} className="grid grid-cols-[1fr_9rem_2.5rem] gap-2">
                  <TextInput
                    id={`folder-${i}`}
                    label={`Folder ${i + 1}`}
                    value={project.folder}
                    onChange={(folder) => updateProject(i, { folder })}
                    placeholder={i === 0 ? 'All folders' : 'e.g. Api'}
                  />
                  <TextInput
                    id={`script-${i}`}
                    label={`Build script for folder ${i + 1}`}
                    value={isDotnet(project.folder) ? '' : project.script}
                    onChange={(script) => updateProject(i, { script })}
                    placeholder={isDotnet(project.folder) ? 'Not needed' : DEFAULT_SCRIPT}
                    disabled={isDotnet(project.folder)}
                  />
                  <button
                    type="button"
                    onClick={() => removeProject(i)}
                    disabled={projects.length === 1 && !project.folder && !project.script}
                    aria-label={`Remove folder ${i + 1}`}
                    title="Remove"
                    className="grid h-9 place-items-center rounded-lg text-faint transition hover:bg-field hover:text-ink disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:bg-transparent"
                  >
                    <CrossIcon size={13} />
                  </button>
                </div>
              ))}
            </div>
            <div className="mt-2.5 flex items-center justify-between gap-4">
              <p className="text-[12.5px] text-faint">Only listed folders are downloaded. Leave empty for everything.</p>
              <button type="button" onClick={addProject} className={quietButton + ' shrink-0'}>
                <PlusIcon />
                Add folder
              </button>
            </div>
            <div className="mt-3 border-t border-line pt-3">
              <Toggle checked={cleanBuild} onChange={setCleanBuild}>
                Clean build <span className="text-faint">(download all packages again; slower)</span>
              </Toggle>
            </div>
          </Section>

          <Section step={3} title="Save the build to">
            <div className="flex gap-2">
              <TextInput id="out" label="Save the build to" value={outputDir} onChange={setOutputDir} placeholder="\\SERVER\Builds" />
              <button type="button" onClick={browse} className={fieldButton}>
                <FolderIcon />
                Browse
              </button>
            </div>
            <p className="mt-1.5 text-[12.5px] text-faint">Every build gets its own numbered folder inside this one.</p>

            {networkFolder && (
              <div ref={signInBox} className="mt-3 border-t border-line pt-3">
                <Toggle checked={shareSignIn} onChange={showSignIn}>
                  This folder needs its own sign-in
                </Toggle>
                {shareSignIn && (
                  <div className="mt-3 grid grid-cols-2 gap-3">
                    <Field label="Folder user name" htmlFor="share-user" hint="For example SERVER\name">
                      <TextInput id="share-user" value={shareUsername} onChange={setShareUsername} />
                    </Field>
                    <Field label="Folder password" htmlFor="share-pass">
                      <PasswordInput id="share-pass" value={sharePassword} onChange={setSharePassword} />
                    </Field>
                  </div>
                )}
              </div>
            )}
          </Section>
        </Body>

        <Footer>
          {formError && <Notice tone="bad">{formError}</Notice>}
          <button type="submit" disabled={svnMissing || starting} className={primaryButton + ' w-full'}>
            {starting ? <Spinner size={14} /> : <PlayIcon />}
            Start build
          </button>
        </Footer>
      </form>
    )
  }

  if (view === 'running') {
    return (
      <div className="flex h-full flex-col">
        <Body>
          <header className="flex items-center justify-between gap-4 px-1 pb-1">
            <div>
              <h1 className="font-display text-[18px] leading-tight font-semibold">{stopping ? 'Stopping…' : 'Building your project'}</h1>
              <p className="text-[13px] text-muted">This can take several minutes. Keep this window open.</p>
            </div>
            <span className="flex h-7 items-center gap-2 rounded-full border border-line bg-surface px-3 text-[13px] font-medium text-muted">
              <span className="size-1.5 animate-pulse rounded-full bg-accent" />
              <Elapsed since={startedAt} />
            </span>
          </header>

          <StepList activeStage={stage} detail={detail} />
          <LogDetails lines={logLines} />
        </Body>

        <Footer>
          <button type="button" onClick={stopBuild} disabled={stopping} className={secondaryButton + ' w-full'}>
            <StopIcon />
            Stop build
          </button>
        </Footer>
      </div>
    )
  }

  if (!outcome) return null

  if (outcome.status === 'success') {
    const built = outcome.projects.filter((p) => p.success)
    return (
      <div className="flex h-full flex-col">
        <Body>
          <div className="rounded-xl border border-line bg-surface p-5 shadow-card">
            <div className="flex items-center gap-4">
              <span className="grid size-12 shrink-0 place-items-center rounded-full bg-good-soft text-good">
                <CheckIcon size={24} />
              </span>
              <div>
                <div className="text-[13px] font-medium text-good">Build finished</div>
                <h1 className="font-display text-[28px] leading-tight font-semibold tabular-nums">Build {outcome.buildNumber} is ready</h1>
              </div>
            </div>
            <dl className="mt-4 grid grid-cols-2 gap-3 border-t border-line pt-4 text-[13px]">
              <div>
                <dt className="text-faint">SVN revision</dt>
                <dd className="mt-0.5 font-medium tabular-nums">{outcome.revision}</dd>
              </div>
              <div>
                <dt className="text-faint">Projects built</dt>
                <dd className="mt-0.5 font-medium tabular-nums">{built.length}</dd>
              </div>
            </dl>
          </div>

          <div className="rounded-xl border border-line bg-surface shadow-card">
            <div className="border-b border-line p-4">
              <div className="flex items-center justify-between gap-3">
                <span className="text-[12.5px] font-medium text-muted">Saved in</span>
                <CopyButton text={outcome.buildDir} label="Copy path" />
              </div>
              <div className="selectable mt-1.5 font-mono text-[12.5px] break-all">{outcome.buildDir}</div>
            </div>
            <ul>
              {built.map((p) => (
                <li key={p.name} className="flex items-center justify-between gap-3 border-b border-line px-4 py-2.5 last:border-b-0">
                  <span className="flex items-center gap-2.5 font-medium">
                    <span className="text-faint">
                      <FolderIcon />
                    </span>
                    {p.name}
                  </span>
                  <span className="text-[13px] text-muted">{PROJECT_KINDS[p.type] ?? p.type}</span>
                </li>
              ))}
            </ul>
          </div>

          <LogDetails lines={logLines} />
        </Body>

        <Footer>
          {openError && <Notice tone="bad">{openError}</Notice>}
          <div className="grid grid-cols-2 gap-2.5">
            <button type="button" onClick={() => setView('form')} className={secondaryButton}>
              Start another build
            </button>
            <button type="button" onClick={() => OpenFolder(outcome.buildDir).catch((err) => setOpenError(String(err)))} className={primaryButton}>
              <FolderIcon />
              Open folder
            </button>
          </div>
        </Footer>
      </div>
    )
  }

  const failed = outcome.status === 'failed'
  return (
    <div className="flex h-full flex-col">
      <Body>
        <div className="rounded-xl border border-line bg-surface p-5 shadow-card">
          <div className="flex items-start gap-4">
            <span className={'grid size-12 shrink-0 place-items-center rounded-full ' + (failed ? 'bg-bad-soft text-bad' : 'bg-accent-soft text-accent')}>
              {failed ? <CrossIcon size={20} /> : <StopIcon />}
            </span>
            <div className="min-w-0">
              <h1 className="font-display text-[20px] leading-tight font-semibold">
                {failed && "The build didn't finish"}
                {outcome.status === 'cancelled' && 'Build stopped'}
                {outcome.status === 'nothing' && 'Nothing to build'}
              </h1>
              <p className="mt-1.5 text-muted">{outcome.message}</p>
            </div>
          </div>
        </div>

        {failed && <StepList activeStage={outcome.failedStage} detail="" failed />}
        {failed && outcome.details && !outcome.message.includes(outcome.details) && (
          <div className="rounded-xl border border-line bg-surface p-4 shadow-card">
            <div className="flex items-center justify-between gap-3">
              <span className="text-[12.5px] font-medium text-muted">Message to pass on to a developer</span>
              <CopyButton text={outcome.details} />
            </div>
            <div className="selectable mt-2 max-h-28 overflow-y-auto font-mono text-[12.5px] wrap-break-word whitespace-pre-wrap text-muted">
              {outcome.details}
            </div>
          </div>
        )}

        <LogDetails lines={logLines} />
      </Body>

      <Footer>
        <button type="button" onClick={() => setView('form')} className={primaryButton + ' w-full'}>
          {failed ? 'Back to try again' : 'Back'}
        </button>
      </Footer>
    </div>
  )
}

function Body(props: { children: ReactNode }) {
  return <main className="view-enter flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto p-5 pb-4">{props.children}</main>
}

function Footer(props: { children: ReactNode }) {
  return <footer className="flex flex-col gap-3 border-t border-line bg-surface px-5 py-4">{props.children}</footer>
}

function StepList(props: { activeStage: string; detail: string; failed?: boolean }) {
  const activeIndex = STEPS.findIndex((s) => s.key === props.activeStage)

  return (
    <ol className="rounded-xl border border-line bg-surface p-4 shadow-card">
      {STEPS.map((step, i) => {
        let state: StepState = 'waiting'
        if (i < activeIndex) state = 'done'
        else if (i === activeIndex) state = props.failed ? 'failed' : 'active'
        const last = i === STEPS.length - 1

        return (
          <li key={step.key} className="flex gap-3">
            <div className="flex flex-col items-center">
              <StepMarker state={state} />
              {!last && <span className={'my-1 w-px flex-1 ' + (state === 'done' ? 'bg-good' : 'bg-line')} />}
            </div>
            <div className={'min-w-0 ' + (last ? '' : 'pb-5')}>
              <div className={'leading-6 ' + (state === 'waiting' ? 'text-faint' : 'font-medium')}>{step.label}</div>
              {state === 'active' && props.detail && <div className="truncate text-[13px] text-muted">{props.detail}</div>}
              {state === 'failed' && <div className="text-[13px] text-bad">Stopped here</div>}
            </div>
          </li>
        )
      })}
    </ol>
  )
}

function StepMarker(props: { state: StepState }) {
  const base = 'grid size-6 shrink-0 place-items-center rounded-full'
  switch (props.state) {
    case 'done':
      return (
        <span className={base + ' bg-good-soft text-good'}>
          <CheckIcon size={13} />
        </span>
      )
    case 'failed':
      return (
        <span className={base + ' bg-bad-soft text-bad'}>
          <CrossIcon size={11} />
        </span>
      )
    case 'active':
      return (
        <span className={base + ' bg-accent-soft text-accent'}>
          <Spinner size={12} />
        </span>
      )
    default:
      return <span className={base + ' border border-line-strong'} />
  }
}
