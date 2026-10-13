# CI/CD Engine — Documentation

A lightweight CI/CD engine for SVN repositories. Automatically detects, builds, and publishes React, Next.js, and .NET projects with zero pipeline scripting.

---

## Table of Contents

1. [Commands](#commands)
2. [YAML Pipeline Configuration](#yaml-pipeline-configuration)
3. [Engine Configuration File](#engine-configuration-file)
4. [Pipeline Resolution Order](#pipeline-resolution-order)
5. [Project Types & Builders](#project-types--builders)
6. [Build Script Override](#build-script-override)
7. [Deployment Structure](#deployment-structure)
8. [Output Folder & Build Numbers](#output-folder--build-numbers)
9. [Desktop App (SVN Builder)](#desktop-app-svn-builder)
10. [Examples](#examples)

---

## Commands

### `cicd run`

Triggers a pipeline build. Accepts either a YAML pipeline file or a bare SVN URL.

```powershell
# Run using an explicit YAML pipeline file
.\cicd.exe run <cicd.yaml> --username <user> --password "<pass>"

# Run using a direct SVN URL (auto-discovery mode)
.\cicd.exe run <svn-url> --username <user> --password "<pass>"
```

**Flags**

| Flag               | Type   | Default | Description                            |
| ------------------ | ------ | ------- | -------------------------------------- |
| `--username`       | string | —       | SVN repository username                |
| `--password`       | string | —       | SVN repository password                |
| `--revision`       | string | `HEAD`  | Target SVN revision to build           |
| `--config`         | string | —       | Path to a JSON engine config file      |
| `--keep-workspace` | bool   | `false` | Retain workspace directory after build |
| `--output`         | string | —       | Folder or network share that receives each build under a unique build number (see [Output Folder & Build Numbers](#output-folder--build-numbers)) |
| `--workspace`      | string | —       | Directory where the checkout workspace is created |
| `--clean-workspace`| bool   | `false` | Remove workspace directory after build |
| `--project`        | string | all     | Comma-separated repository folders to build, e.g. `Client,Api`. Only these folders are downloaded from SVN (see note below) |
| `--script`         | string | `build` | npm script used to build Node projects that have no `build.script` in YAML |
| `--reuse-deps`     | bool   | `false` | Keep each Node project's `node_modules` between runs (in `<workspace>\.deps`) so repeat builds skip most of the install |
| `--clean-deps`     | bool   | `false` | With `--reuse-deps`: discard the kept `node_modules` of this run's projects first and install from scratch |

**Selected folders and download size.** With `--project` (or the desktop app's folder list), the engine downloads only those folders plus the files in the top folder of the address, not the whole tree. The whole tree is still downloaded when a pipeline YAML file is passed on the command line, when the repository has a `cicd.yaml` in its top folder (it may point at other folders), or when a listed name is not a folder in the repository.

**Examples**

```powershell
# Build from a YAML file
.\cicd.exe run tpp.yaml --username anandhu --password "secret"

# Build a specific SVN revision
.\cicd.exe run tpp.yaml --username anandhu --password "secret" --revision 298

# Keep the workspace for inspection after build
.\cicd.exe run tpp.yaml --username anandhu --password "secret" --keep-workspace

# Use a custom engine config file
.\cicd.exe run tpp.yaml --username anandhu --password "secret" --config cicd-config.json

# Run directly from an SVN URL (no YAML file needed)
.\cicd.exe run https://svn.example.com/repos/MyProject --username anandhu --password "secret"

# Check out to the temp folder, build, and save the result to a server share under a new build number
.\cicd.exe run https://svn.example.com/repos/MyProject/trunk --username anandhu --password "secret" `
  --output \\SERVER\Builds --workspace $env:TEMP\cicd --clean-workspace

# Build only the Client folder, using "npm run build:prod" instead of "npm run build"
.\cicd.exe run https://svn.example.com/repos/MyProject/trunk --username anandhu --password "secret" `
  --project Client --script build:prod
```

---

### `cicd history`

Displays previously completed build jobs.

```powershell
.\cicd.exe history
```

**Output fields:** Job ID, Repository, Revision, Status, Started, Duration, Artifacts count.

---

### `cicd status`

Displays currently running or queued build jobs.

```powershell
.\cicd.exe status
```

**Output fields:** Job ID, Repository, Revision, Status.

---

### `cicd doctor`

Validates the environment — checks that all required tools are installed and reachable.

```powershell
.\cicd.exe doctor

# With a custom engine config
.\cicd.exe doctor --config cicd-config.json
```

Checks for: `svn`, `node`, `npm`, `dotnet`.

---

### `cicd help`

Prints usage information.

```powershell
.\cicd.exe help
.\cicd.exe --help
.\cicd.exe -h
```

---

## YAML Pipeline Configuration

The YAML file describes **what** to build and **where** to deploy it. It does **not** contain shell commands.

### Full Schema

```yaml
version: 1

# ── Repository ─────────────────────────────────────────────
repository:
  type: svn
  url: https://svn.example.com/repos/REPONAME/PROJECT

# ── Build Strategy ─────────────────────────────────────────
build:
  # "fail-fast"  → stop on first project failure (default)
  # "continue"   → attempt remaining projects even after a failure
  strategy: fail-fast

  # Which projects to build, in this order.
  # Must match the 'name' values in the projects list below.
  targets:
    - Client
    - API

# ── Deployment ─────────────────────────────────────────────
deployment:
  # true  → overwrite existing deployment folder (default)
  # false → skip publish if destination already exists
  replaceExisting: true

  # "all-or-nothing"   → only publish if every target succeeded (default)
  # "successful-only"  → publish whichever targets built successfully
  publish: all-or-nothing

# ── Projects ───────────────────────────────────────────────
projects:
  - name: Client
    path: Client # Subfolder path inside SVN repo ("" = root)
    output: D:\CI\Deployments\TPP\Client # Permanent deployment destination

  - name: API
    path: Api
    output: D:\CI\Deployments\TPP\API

  # .NET project — no build section required
  - name: Scheduler
    path: SFTPDailyScheduler
    output: D:\CI\Deployments\TPP\Scheduler

  # React/Next project with a custom npm build script
  - name: Dashboard
    path: Dashboard
    output: D:\CI\Deployments\TPP\Dashboard
    build:
      script: deploy # runs: npm run deploy  (default is: npm run build)
```

### YAML Field Reference

| Field                        | Required | Default          | Description                                    |
| ---------------------------- | -------- | ---------------- | ---------------------------------------------- |
| `version`                    | ✅       | —                | Must be `1`                                    |
| `repository.type`            | ✅       | —                | Source control type. Currently `svn`           |
| `repository.url`             | ✅       | —                | Full SVN URL to the project folder             |
| `build.strategy`             | ❌       | `fail-fast`      | `fail-fast` or `continue`                      |
| `build.targets`              | ❌       | all projects     | Ordered list of project names to build         |
| `deployment.replaceExisting` | ❌       | `true`           | Overwrite existing output folder               |
| `deployment.publish`         | ❌       | `all-or-nothing` | `all-or-nothing` or `successful-only`          |
| `projects[].name`            | ✅       | —                | Display name. Must match a `targets` entry     |
| `projects[].path`            | ✅       | —                | Subfolder inside the SVN checkout              |
| `projects[].output`          | ✅       | —                | Absolute path to deployment destination        |
| `projects[].build.script`    | ❌       | `build`          | npm/pnpm/yarn script name (Node projects only) |

---

## Engine Configuration File

Optional JSON file that controls engine-level behaviour. Pass it with `--config`.

### Full Example

```json
{
  "workspace": {
    "root": "D:\\CI\\.workspaces",
    "cleanup": false
  },
  "deployment": {
    "directory": "D:\\CI\\.deployments",
    "replace_existing": true
  },
  "logging": {
    "level": "INFO"
  },
  "retry": {
    "enabled": true,
    "count": 3,
    "backoff_seconds": 1
  },
  "cleanup": {
    "enabled": true
  },
  "parallel_build_count": 1,
  "credential_provider": "env"
}
```

### Config Field Reference

| Field                         | Default          | Description                                                    |
| ----------------------------- | ---------------- | -------------------------------------------------------------- |
| `workspace.root`              | `.\.workspaces`  | Directory where build workspaces are created                   |
| `workspace.cleanup`           | `false`          | Delete workspace after build (`true` = delete, `false` = keep) |
| `deployment.directory`        | `.\.deployments` | Fallback deployment directory when YAML `output` is not set    |
| `deployment.replace_existing` | `true`           | Overwrite existing deployment outputs                          |
| `logging.level`               | `INFO`           | Log verbosity: `DEBUG`, `INFO`, `WARN`, `ERROR`                |
| `retry.enabled`               | `true`           | Retry transient operations (e.g. SVN checkout)                 |
| `retry.count`                 | `3`              | Number of retry attempts                                       |
| `retry.backoff_seconds`       | `1`              | Wait time between retries (seconds)                            |
| `cleanup.enabled`             | `true`           | Remove `bin/`, `obj/` and other generated folders before build |
| `parallel_build_count`        | `1`              | Maximum concurrent project builds                              |
| `credential_provider`         | `env`            | `env` · `static` · `wincred`                                   |

---

## Pipeline Resolution Order

When `cicd run` is invoked, the engine resolves **how** to build using this priority:

```
1. Explicit YAML file passed on command line
       .\cicd.exe run tpp.yaml

2. cicd.yaml / cicd.yml found in the SVN repository root (auto-discovery)
       .\cicd.exe run https://svn.example.com/repos/MyProject
       → checks out → finds cicd.yaml → uses it

3. No YAML file found anywhere → legacy auto-scan mode
       → scans the workspace → builds every detected project automatically
```

---

## Project Types & Builders

The engine detects project types automatically from the file system. No manual configuration is needed.

| Detector    | Trigger                          | Build Steps                         |
| ----------- | -------------------------------- | ----------------------------------- |
| **React**   | `package.json` + Vite config     | `npm install` → `npm run build`     |
| **Next.js** | `package.json` + `next.config.*` | `npm install` → `npm run build`     |
| **.NET**    | `*.csproj` or `*.sln`            | `dotnet restore` → `dotnet publish` |

Package manager (`npm` / `pnpm` / `yarn`) is detected automatically from lockfiles.

---

## Build Script Override

For Node (React / Next.js) projects, the default build script is `build` (`npm run build`).
Override it per-project in the YAML when your `package.json` uses a different script name:

```yaml
projects:
  - name: Client
    path: Client
    output: D:\CI\Deployments\TPP\Client
    # no build section → runs: npm run build

  - name: Dashboard
    path: Dashboard
    output: D:\CI\Deployments\TPP\Dashboard
    build:
      script: deploy # runs: npm run deploy

  - name: AdminPanel
    path: AdminPanel
    output: D:\CI\Deployments\TPP\AdminPanel
    build:
      script: "build:prod" # runs: npm run build:prod
```

The engine validates that the configured script exists in `package.json` **before** the build starts. If it does not exist, you will see:

```
Pipeline Validation Failed

Project:
Dashboard

Configured script:
deploy-prod

Available scripts:
  build
  deploy
  lint
  test
```

---

## Deployment Structure

Published outputs are organized by repository name and project name:

```
<output path>
└── <RepoName>/
    ├── Client/      ← dist/ contents
    └── API/         ← dotnet publish output
```

If `output` is specified in the YAML, it is used directly as the destination.
After publishing, heavy workspace folders (`node_modules`, `dist`, `bin`, `obj`) are automatically cleaned from the workspace to save disk space.

---

## Output Folder & Build Numbers

Pass `--output <folder>` (or use the desktop app) to collect every run in one place, typically a shared folder on a Windows server. The checkout and build still happen on the machine that runs the engine; only the finished output is copied to the folder.

```
\\SERVER\Builds
└── MyProject/                 ← application name taken from the SVN URL
    ├── Build-0001/
    │   ├── build-info.json    ← build number, SVN revision, who built it, when
    │   ├── Client/            ← dist/ contents
    │   └── API/               ← dotnet publish output
    └── Build-0002/
        └── ...
```

- The build number is the next free number in the application's folder. It is reserved by creating the folder, so two people building at the same time never get the same number.
- A number is only used when the build succeeds. Failed or cancelled builds leave nothing on the share.
- `--output` overrides the `output` paths in the YAML file and the `deployment.directory` config value.
- Branches and tags get their own sequence: `.../MyProject/branches/v2` is saved under `MyProject_v2`.

---

## Desktop App (SVN Builder)

A small [Wails](https://wails.io) window in `desktop/` for people who should not need the command line. The user enters the SVN address, their SVN user name and password, and the folder to save to, then presses **Start build**. The **Folders to build** list narrows the build: each row is a folder inside the project plus an optional build script for it (the npm script for website projects, `build` when left empty). **+ Add folder** adds a row, and the folders are built in the order listed. Leaving the list empty builds every project found. **Find folders in SVN** asks the server which folders can be built (using the address, user name and password already typed, without downloading the project) and shows them as buttons that add or remove a folder from the list. It checks out into `%TEMP%\SvnBuilder`, builds, saves to the chosen folder under a new build number, and deletes the temporary checkout.

```powershell
cd desktop

# Build desktop\build\bin\svn-builder.exe
wails build

# Run with live reload while developing
wails dev
```

The PC running the app needs the same tools as the CLI: `svn` on the PATH, plus `node`/`npm` for web projects and `dotnet` for .NET projects. The app shows a warning on its first screen when one is missing.

If the save folder is a network folder (`\\SERVER\Builds`) that needs a different account from the one logged on to Windows, turn on **This folder needs its own sign-in** and enter that account. The app signs in to the share before the build starts, the same way File Explorer does, and the connection lasts until the user logs off Windows. The app also checks that it can write to the folder before starting, so an access problem shows up immediately and not after a long build. The CLI has no equivalent option; connect the share first with `net use \\SERVER\Builds /user:NAME`.

The last SVN address, user names and folders are remembered in `%APPDATA%\SvnBuilder\settings.json`. Passwords are never stored.

The SVN user name and password sit under **SVN sign-in**, closed by default. Left empty, the app uses the SVN sign-in already saved on the PC (the one TortoiseSVN and the `svn` command share), which is enough on most machines. The section opens by itself when the server asks for a sign-in.

The arrow in the address box opens a searchable list of the SVN addresses used on this PC (up to 20, most recent first; the × forgets one). The app does not ask the server for its repositories: that needs `SVNListParentPath On` on an Apache SVN server, and many servers have it off. After **Find folders in SVN**, the buildable folders are chosen from a searchable drop-down in the same style. Folders whose name starts with `GRAMPRO.` (the shared framework projects) are left out of that list; the **Hide GRAMPRO.\* framework folders** switch under **Advanced options** (below the folder rows, closed by default) shows them again, and its setting is remembered. **Clean build** is under Advanced options too.

**Settings** (the gear at the top right) has an **Appearance** switch (Same as Windows, Light, Dark) that is remembered between sessions, and a **Working drive** choice for PCs whose system drive is short on space. It lists the internal drives with their free space; choosing one moves downloads and builds to `<drive>:\SvnBuilder\workspaces` and the build history and logs to `<drive>:\SvnBuilder`. "Automatic" keeps the defaults (`%TEMP%\SvnBuilder` and `%APPDATA%\SvnBuilder`). The main screen shows a warning when the working drive has under 2 GB free, and a build is not started with under 500 MB free. If the settings file cannot be written to `%APPDATA%` (for example because that drive is full), it is saved as `svn-builder-settings.json` next to the executable.

The app keeps each website project's installed packages (`node_modules`) on the working drive after a build and moves them back in for the next build of that project, so the install step only has to check them against `package.json`. The **Clean build** switch on the main screen discards them for one build and installs everything again; **Settings → Saved packages → Clear** removes them all, and the setting next to it chooses how long they are kept after a project's last build (3, 7 or 30 days, or always; 7 days by default). Expired ones are deleted when the app starts and after each build. npm installs run with `--prefer-offline --no-audit --no-fund`.

When a network save folder needs its own sign-in and Windows already holds a connection to that share under another account (for example after opening it in File Explorer), the app drops that connection and signs in with the account entered. Mapped drives and connections to other shares on the same server are left alone.

The result screen appears as soon as the output is saved; the temporary download (which can hold tens of thousands of dependency files) is deleted in the background afterwards. Temporary downloads left behind by a crash or a forced close are deleted the next time the app starts. Only one copy of the app runs at a time; starting it again brings the open window forward.

### Versions and updates

A release has a version number, shown in **Settings → About**, and knows the folder its updates are published to. When it starts it looks in that folder; if a newer version is there, a notice with **Update now** appears on the main screen and in About. Updating downloads the new file next to the current one, checks it, swaps it in and restarts the app. No installer and no administrator rights are needed, as long as the user can write to the folder the app sits in.

Publishing a release is one command, run from the repository:

```powershell
# Once per publisher: creates the signing key and builds its public half into the app
go run ./cmd/release keygen

# Every release
go run ./cmd/release -version 1.2.0 -to \\SERVER\Share\SvnBuilder -notes "What changed"
```

It builds the app with the version and release folder baked in, signs it, and copies `svn-builder.exe` and `latest.json` to the folder. People install the app the first time by copying `svn-builder.exe` out of that folder; after that it updates itself. A version number can only go up.

Releases are signed. The private key is saved as `svn-builder-release.key` in the publisher's user folder, outside the repository; the app carries the public key (`desktop/update_key.go`). An installed copy ignores a manifest that is not signed with that key and refuses a file that does not match the signed checksum, so write access to the release folder is not enough to push software to users. Keep a backup of the key file: without it, new releases are not accepted by copies already installed, and they would have to be replaced by hand.

A plain `wails build` produces a development build (version `dev`), which never looks for updates.

The window has a fixed size (580 × 800) and cannot be resized or maximized; on a shorter screen it shrinks to fit and the form scrolls.

> Wails CLI 2.12 cannot generate bindings with Go 1.27. If `wails build` stops with `package "bytes" without types was imported`, run it with an older toolchain: `$env:GOTOOLCHAIN = 'local'; wails build`.

---

## Examples

### Minimal YAML — single React project

```yaml
version: 1
repository:
  type: svn
  url: https://svn.example.com/repos/TPP/MY-PROJECT
build:
  strategy: fail-fast
  targets:
    - Client
deployment:
  publish: all-or-nothing
projects:
  - name: Client
    path: Client
    output: D:\CI\Deployments\MY-PROJECT\Client
```

### Full YAML — React + .NET with custom build script

```yaml
version: 1
repository:
  type: svn
  url: https://svn.example.com/repos/UAM/MY-APP
build:
  strategy: fail-fast
  targets:
    - Client
    - API
deployment:
  replaceExisting: true
  publish: all-or-nothing
projects:
  - name: Client
    path: Client
    output: D:\CI\Deployments\MY-APP\Client
    build:
      script: deploy

  - name: API
    path: Api
    output: D:\CI\Deployments\MY-APP\API
```

### Run commands quick reference

```powershell
# Standard YAML-driven build
.\cicd.exe run project.yaml --username user --password "pass"

# Build a specific SVN revision
.\cicd.exe run project.yaml --username user --password "pass" --revision 350

# Keep workspace for post-build inspection
.\cicd.exe run project.yaml --username user --password "pass" --keep-workspace

# Direct SVN URL build (no YAML required)
.\cicd.exe run https://svn.example.com/repos/UAM/MY-APP --username user --password "pass"

# Check build history
.\cicd.exe history

# Check running jobs
.\cicd.exe status

# Validate environment tools
.\cicd.exe doctor
```
