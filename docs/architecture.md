[<- Configuration](configuration.md) | [Back to README](../README.md)

# Architecture

## Overview

Relix is a single-package Go application built on the [Bubble Tea](https://github.com/charmbracelet/bubbletea) TUI framework, following the Elm architecture (Model-Update-View). The application is organized as a screen-based state machine where each screen has dedicated update and view handlers.

## Screen Flow

```
screenLoading
    |
screenAuth ---------> screenHome
                         |
           +-------------+-------------+
           |                           |
    screenMain (MR list)        screenHistoryList
           |                           |
    screenEnvSelect             screenHistoryDetail
           |
    screenVersion
           |
    screenSourceBranch
           |
    screenRootMerge
           |
    screenConfirm
           |
    screenRelease
           |
    screenHome (complete)
```

## Project Structure

All source files are in the root package (`package main`). The codebase is organized by responsibility:

### Core

| File | Lines | Purpose |
|------|-------|---------|
| `main.go` | ~100 | Entry point, CLI flags, program init |
| `model.go` | ~600 | Central model, Update/View routing, modal management |
| `types.go` | ~400 | All type definitions, screen constants, message types |

### Screens

Each screen has an `update{Screen}` function (handles key events and messages) and a `view{Screen}` function (renders the UI):

| File | Screen | Purpose |
|------|--------|---------|
| `auth_screen.go` | `screenAuth` | GitLab credential input |
| `home_screen.go` | `screenHome` | Project info, actions menu |
| `mrs_screen.go` | `screenMain` | MR list with selection |
| `environment_screen.go` | `screenEnvSelect` | Environment picker |
| `version_screen.go` | `screenVersion` | Semantic version input |
| `source_branch_screen.go` | `screenSourceBranch` | Source branch config |
| `root_merge_screen.go` | `screenRootMerge` | Merge-back strategy |
| `confirm_screen.go` | `screenConfirm` | Release summary review |
| `release_screen.go` | `screenRelease` | Release execution (largest file) |
| `history_list_screen.go` | `screenHistoryList` | Release history browser |
| `history_detail_screen.go` | `screenHistoryDetail` | Release detail view |
| `error_screen.go` | `screenError` | Error display |

### Infrastructure

| File | Purpose |
|------|---------|
| `gitlab.go` | GitLab API client (projects, MRs, pipelines, diffs) |
| `git_executor.go` | PTY-based git execution with virtual terminal |
| `config.go` | Config file I/O (`~/.relix/config.json`) |
| `keyring.go` | OS keyring for credential storage |
| `release_history.go` | Release history persistence (index + detail files) |

### UI

| File | Purpose |
|------|---------|
| `styles.go` | Lipgloss style definitions |
| `theme.go` | Dynamic theming with ANSI color remapping |
| `modal.go` | Modal overlay base |
| `command_menu.go` | Command menu (`/` key) |
| `project_selector.go` | Project search/selection modal |
| `open_options_modal.go` | Browser open options |
| `settings_screen.go` | Settings modal (release + theme tabs) |
| `utils.go` | Text wrapping, version parsing, exclusions |

## Key Patterns

### Message-Based Async

All long-running operations (API calls, git commands) return `tea.Cmd` that send typed result messages. Loading states are tracked via boolean flags (`loadingMRs`, `loadingProjects`, etc.) to show spinners.

```
User Action -> tea.Cmd -> Async Operation -> tea.Msg -> Update() -> New State
```

### Modal System

Modals overlay the base screen via boolean flags (`showCommandMenu`, `showProjectSelector`, `showSettings`). Key events are routed to the active modal first, then to the screen handler. `closeAllModals()` centralizes cleanup.

### Release State Machine

The release process (`release_screen.go`) is a multi-step state machine tracked by `ReleaseStep`. State is persisted to `~/.relix/release.json` after each step for crash recovery. The flow:

1. Steps execute git commands via `GitExecutor`
2. `releaseStepCompleteMsg` signals step completion
3. Next step starts automatically (or waits for user input)
4. On conflict/error, process pauses for user intervention
5. On completion, state is saved to history and release file is deleted

### Git Executor

Git commands run in a PTY (pseudo-terminal) for colored output. The `VirtualTerminal` (backed by vt10x) parses ANSI escape codes and maintains a cell grid. Output is streamed to the UI at 20 FPS via `releaseScreenMsg`.

```
Git Command -> PTY -> Raw Bytes -> VirtualTerminal -> ANSI Parsing -> Cell Grid -> UI
```

### Two-Tier History

Release history uses a lightweight index file for fast list rendering and separate detail files for full release data (terminal output, MR metadata, commit SHAs).

## See Also

- [Configuration](configuration.md) -- config structure and theme system
- [Usage Guide](usage.md) -- the release workflow from a user perspective
