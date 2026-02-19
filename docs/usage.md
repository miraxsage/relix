[<- Getting Started](getting-started.md) | [Back to README](../README.md) | [Configuration ->](configuration.md)

# Usage Guide

## Release Workflow

Relix guides you through a structured release process:

**Select MRs -> Choose Environment -> Set Version -> Configure Branches -> Confirm -> Release**

### 1. Project Selection

Select the GitLab project you want to manage. Relix filters projects as you type.

| Key | Action |
|-----|--------|
| `Up` / `Down` or `Ctrl+n` / `Ctrl+p` | Navigate list |
| `Enter` | Select project |

### 2. Select Merge Requests

Browse open MRs for the selected project. Each MR shows diff stats, discussion count, and conflict status.

| Key | Action |
|-----|--------|
| `Space` | Toggle MR selection |
| `Enter` | Confirm selection and proceed |
| `o` | Open highlighted MR in browser |
| `r` | Refresh MR list |
| `d` / `u` | Scroll details view down/up |

### 3. Choose Environment

Select the target environment for the release. Default environments:

| Environment | Branch |
|-------------|--------|
| DEVELOP | `develop` |
| TEST | `testing` |
| STAGE | `stable` |
| PROD | `master` |

Environment names and branches are fully customizable in [Settings](configuration.md#environments).

### 4. Versioning

Enter a semantic version for the release (e.g., `1.2.0`). Relix validates the `X.Y.Z` format.

### 5. Source Branch

Configure the source branch name for the release. This is the branch where all selected MR branches will be merged before creating the environment release branch.

### 6. Root Merge

Choose whether to merge back to the base branch after release:

- **Enabled**: After release, merges the source branch back to the base branch, tags it, and pushes to develop
- **Disabled**: Only tags the source branch without merging back

### 7. Confirmation

Review all release details before execution:
- Selected MRs with branch names
- Target environment and branch
- Version number
- Source and base branches
- Root merge strategy

### 8. Release Execution

Relix executes the release steps automatically, showing real-time terminal output:

1. **Git Fetch** -- fetches remote updates
2. **Checkout Source** -- creates or restores the source branch
3. **Merge Branches** -- merges each selected MR branch sequentially
4. **Checkout Environment** -- creates the environment release branch
5. **Copy Content** -- replaces env content with source, applies exclusions
6. **Commit** -- creates the release commit
7. **Push & Create MR** -- pushes to remote and creates the GitLab MR
8. **Push Root Branches** -- tags and merges back to base/develop (if enabled)

#### Conflict Handling

If a merge conflict occurs, the process pauses. Resolve the conflict in your terminal, then press **Retry** to continue.

#### Abort

You can abort the process at any time. Relix attempts to reset your git state and saves the release to history as "aborted".

#### Crash Recovery

Release state is saved automatically after each step to `~/.relix/release.json`. If Relix crashes or is closed mid-release, it will offer to resume on next launch.

## Release History

Access release history from the Home screen. History shows:

- Release date and version
- Target environment
- Status (completed/aborted)
- MR details with URLs, IIDs, and commit SHAs
- Full terminal output captured during release

| Key | Action |
|-----|--------|
| `Enter` | View release details |
| `Space` | Toggle selection (for deletion) |
| `o` | Open MR in browser |
| `d` | Delete selected entries |

## Global Shortcuts

| Key | Action |
|-----|--------|
| `/` | Open Command Menu (Settings, Help, etc.) |
| `Ctrl+c` | Quit application |
| `Esc` | Go back / Close modal |

## Pipeline Monitoring

After creating the release MR, Relix monitors the GitLab pipeline:

- Polls every 7 seconds for status updates
- Shows job statuses in the UI
- Sends macOS native notifications on pipeline completion (success or failure)
- Opens the MR in your browser automatically

## See Also

- [Getting Started](getting-started.md) -- installation and first run
- [Configuration](configuration.md) -- environment and theme customization
- [Architecture](architecture.md) -- how Relix works internally
