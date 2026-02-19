# Relix - GitLab Release Manager TUI

> Automate complex release workflows from your terminal.

Relix is an interactive TUI tool that streamlines GitLab release management -- select MRs, target an environment, set a version, and let it handle the git operations, MR creation, and pipeline monitoring.

## Key Features

- **Interactive MR Selection** -- browse, filter, and select multiple Merge Requests with diff stats and conflict detection
- **Environment Targeting** -- release to DEVELOP, TEST, STAGE, or PROD with configurable branch mappings
- **Automated Git Operations** -- merges, checkouts, commits, pushes, and MR creation in one flow
- **Crash Recovery** -- resume interrupted releases exactly where you left off
- **Pipeline Monitoring** -- real-time pipeline status with macOS notifications
- **Secure Credentials** -- stored in your system's keyring, never in plain text
- **Custom Themes** -- full color customization with dynamic ANSI remapping

<img width="800" height="auto" alt="1" src="https://github.com/user-attachments/assets/f7c17425-76ea-40e6-8be8-068e40caf09b" />
<img width="800" height="auto" alt="2" src="https://github.com/user-attachments/assets/21a1c709-6765-420a-96f8-6ad62f07f344" />
<img width="800" height="auto" alt="3" src="https://github.com/user-attachments/assets/7fc9b98f-a943-4f01-b7da-cb5c276e422f" />
<img width="800" height="auto" alt="4" src="https://github.com/user-attachments/assets/662dd777-0de7-4bae-b692-ba3573a1c53d" />
<img width="800" height="auto" alt="5" src="https://github.com/user-attachments/assets/b7ee6141-629d-46f6-9333-d4917c344ae9" />
<img width="800" height="auto" alt="6" src="https://github.com/user-attachments/assets/0f922e06-76e7-45cf-8ca5-71c55a75fc25" />

## Quick Start

```bash
# Build from source
git clone https://github.com/yourusername/relix.git
cd relix
go build -o relix .

# Run in your project directory
./relix
```

**Prerequisites:** Go 1.25+, Git, GitLab PAT with `api` scope.

## Usage

```bash
relix                         # Run in current directory
relix -d /path/to/project     # Specify project directory
relix --version               # Show version
```

On first run, enter your GitLab URL, email, and token. Then select a project and start creating releases.

---

## Documentation

| Guide | Description |
|-------|-------------|
| [Getting Started](docs/getting-started.md) | Installation, authentication, first run |
| [Usage Guide](docs/usage.md) | Full release workflow, shortcuts, history |
| [Configuration](docs/configuration.md) | Environments, themes, exclusions, storage |
| [Architecture](docs/architecture.md) | Project structure, state machine, key patterns |

## Tech Stack

- **Language:** [Go](https://go.dev/)
- **TUI Framework:** [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- **Styling:** [Lipgloss](https://github.com/charmbracelet/lipgloss)
- **Markdown:** [Glamour](https://github.com/charmbracelet/glamour)
- **Keyring:** [go-keyring](https://github.com/zalando/go-keyring)

---

*Relix is a tool for developers who want to stay in the flow and automate the tedious parts of release management.*
