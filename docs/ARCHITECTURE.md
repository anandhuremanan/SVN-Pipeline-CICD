# CI/CD Engine MVP Architecture

## Overview

This project is a lightweight CI/CD engine built in Go for repositories hosted in SVN.

The goal is to automatically:

1. Checkout/update an SVN repository.
2. Scan the repository for supported projects.
3. Build each project.
4. Capture logs and errors.
5. Report build results.

Initially, the engine will be exposed as a CLI.

In the future, the same engine can power:

- Background polling service
- REST API
- Web Dashboard
- Multiple Build Agents

The CLI should only be an entry point. The core build logic must remain independent of the user interface.

---

# Goals

## MVP

- SVN Checkout
- Automatic project detection
- React (Vite) build support
- Next.js build support
- .NET build support
- Build logs
- Build summary
- Temporary workspaces
- Secure credential handling
- Console interface

## Future

- SVN polling
- Post-commit hook integration
- Parallel builds
- Build history
- Web dashboard
- Artifact publishing
- Deployments
- Notifications
- Multiple build agents

---

# High-Level Architecture

```
                 CLI
                  │
                  ▼
          Pipeline Engine
                  │
      ┌───────────┼────────────┐
      ▼           ▼            ▼
 Repository   Workspace    Build Service
   Service     Service
      │                         │
      ▼                         ▼
 Project Scanner          Builder Factory
                                │
           ┌────────────────────┼───────────────────┐
           ▼                    ▼                   ▼
     React Builder       Next Builder       Dotnet Builder
                                │
                                ▼
                         Process Runner
                                │
                                ▼
                          Log Collector
                                │
                                ▼
                             Reporter
```

---

# Pipeline

Every execution follows the same pipeline.

```
Environment Validation

↓

Credential Resolution

↓

Repository Checkout

↓

Workspace Creation

↓

Project Scan

↓

Project Build

↓

Summary Report

↓

Workspace Cleanup
```

Each pipeline stage must be independent.

```go
type Stage interface {
    Execute(ctx *PipelineContext) error
}
```

---

# Project Structure

```
cicd-engine/

├── cmd/
│   └── cicd/
│       └── main.go
│
├── internal/
│
│   ├── cli/
│   │
│   ├── pipeline/
│   │
│   ├── repository/
│   │
│   ├── workspace/
│   │
│   ├── scanner/
│   │
│   ├── builder/
│   │   ├── react/
│   │   ├── next/
│   │   ├── dotnet/
│   │   ├── factory.go
│   │   └── builder.go
│   │
│   ├── process/
│   │
│   ├── credentials/
│   │
│   ├── reporter/
│   │
│   ├── logger/
│   │
│   ├── config/
│   │
│   └── models/
│
├── configs/
│
├── scripts/
│
├── docs/
│
├── go.mod
│
└── README.md
```

---

# Core Components

## CLI

Responsibilities

- Parse commands
- Validate arguments
- Invoke Pipeline Engine

The CLI should never execute build logic directly.

---

## Pipeline Engine

Responsibilities

- Execute stages in order
- Maintain pipeline context
- Handle failures
- Produce final status

This is the heart of the application.

---

## Repository Service

Responsibilities

- SVN checkout
- SVN update
- Revision information
- Repository metadata

Should never contain build logic.

---

## Workspace Service

Responsibilities

- Create temporary workspace
- Cleanup workspace
- Manage build directories

Every build gets its own isolated workspace.

---

## Scanner

Responsibilities

- Traverse repository
- Detect supported project types
- Produce project metadata

Supported project types

- React (Vite)
- Next.js
- .NET

---

## Builder Factory

Given a detected project, returns the appropriate builder.

Example

React Project

↓

React Builder

The pipeline never contains language-specific logic.

---

## Builders

Each builder is responsible for its own build steps.

React Builder

- npm install
- npm run build

Next Builder

- npm install
- npm run build

Dotnet Builder

- dotnet restore
- dotnet build

Builders should never execute shell commands directly.

They must use the Process Runner.

---

## Process Runner

Responsible for

- Running external commands
- Capturing stdout
- Capturing stderr
- Exit codes
- Execution time

Every external process goes through this component.

---

## Reporter

Responsible for

- Console output
- Build summaries
- Error formatting

Future

- HTML reports
- REST responses
- Web dashboard

---

# Security

## Credentials

Username

- Configuration

Password

- OS Credential Manager (future)

MVP

- CLI arguments or environment variables

Passwords must never appear in logs.

---

## Command Execution

Always use native process execution.

Never invoke a shell.

Good

```
exec.Command(...)
```

Bad

```
cmd /C
powershell
bash -c
```

---

## Workspace Isolation

Every build receives a unique temporary workspace.

Example

```
workspace/

    build-001/

    build-002/

    build-003/
```

After completion

Workspace is deleted.

---

## Repository

HTTPS only by default.

Support HTTP only if explicitly configured.

---

## Permissions

The application should run with standard user privileges.

Administrator permissions should never be required.

---

# Future Event Sources

Currently

```
CLI

↓

Pipeline Engine
```

Later

```
CLI

↓

Pipeline Engine

SVN Polling

↓

Pipeline Engine

REST API

↓

Pipeline Engine

Webhook

↓

Pipeline Engine
```

The Pipeline Engine must remain independent of the event source.

---

# Design Principles

- Single Responsibility Principle
- Composition over inheritance
- Dependency Injection where appropriate
- Interface-driven design
- Platform independence
- Secure by default
- Extensible builder architecture
- No business logic inside the CLI
- No direct shell execution
- Clean separation between scanning, building, reporting, and process execution

---

# MVP Scope

Included

- CLI
- SVN checkout
- Workspace creation
- Project detection
- React build
- Next build
- .NET build
- Console reporting
- Build logs
- Workspace cleanup

Excluded

- Deployment
- Docker
- Kubernetes
- Build artifacts
- Notifications
- Dashboard
- Parallel execution
- Multi-agent scheduling
- Distributed builds

```

```
