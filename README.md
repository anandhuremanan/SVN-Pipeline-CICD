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
8. [Examples](#examples)

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
