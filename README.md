# Relix - GitLab Release Manager TUI

Relix is a powerful, interactive terminal user interface (TUI) tool designed to streamline and automate complex release workflows for GitLab projects. Built with Go and the Bubble Tea framework, it provides a robust, visual way to manage Merge Requests, handle versioning, and execute release pipelines directly from your terminal.

## 🚀 Features

-   **Interactive TUI**: Navigate through your release process with a modern, responsive terminal interface.
-   **GitLab Integration**: Seamlessly fetch projects and Merge Requests (MRs) using your GitLab Personal Access Token.
-   **Secure Authentication**: Safely stores your GitLab credentials in your system's keyring.
-   **Smart Release Workflow**:
    -   **Select MRs**: View, filter, and select multiple Merge Requests to include in a release.
    -   **Environment Targeting**: Choose target environments (DEVELOP, TEST, STAGE, PROD) with mapped branches.
    -   **Semantic Versioning**: Validate and apply semantic versioning to your release branches.
-   **Automated Git Operations**: Relix handles the heavy lifting:
    -   Checks out root branches.
    -   Merges selected feature branches (detects conflicts).
    -   Creates environment-specific release branches.
    -   Handles file exclusions (e.g., removing CI files or generated assets).
    -   Creates release commits and pushes to remote.
    -   Opens the final Merge Request for the release.
-   **Resumable Sessions**: State is saved automatically. If a conflict occurs or the app is closed, you can resume exactly where you left off.
-   **Configurable Exclusions**: Define file patterns to automatically exclude from release builds.
  
<img width="800" height="auto" alt="1" src="https://github.com/user-attachments/assets/f7c17425-76ea-40e6-8be8-068e40caf09b" />
<img width="800" height="auto" alt="2" src="https://github.com/user-attachments/assets/21a1c709-6765-420a-96f8-6ad62f07f344" />
<img width="800" height="auto" alt="3" src="https://github.com/user-attachments/assets/7fc9b98f-a943-4f01-b7da-cb5c276e422f" />
<img width="800" height="auto" alt="4" src="https://github.com/user-attachments/assets/662dd777-0de7-4bae-b692-ba3573a1c53d" />
<img width="800" height="auto" alt="5" src="https://github.com/user-attachments/assets/b7ee6141-629d-46f6-9333-d4917c344ae9" />
<img width="800" height="auto" alt="6" src="https://github.com/user-attachments/assets/0f922e06-76e7-45cf-8ca5-71c55a75fc25" />

## 🛠️ Installation

### Prerequisites

-   **Go**: Version 1.25 or higher.
-   **Git**: Installed and available in your PATH.

### Build from Source

```bash
# Clone the repository
git clone https://github.com/yourusername/relix.git
cd relix

# Build the binary
go build -o relix .

# Run the application
./relix
```

Alternatively, you can install it directly:

```bash
go install github.com/yourusername/relix@latest
```

## 📖 Usage Guide

### 1. Authentication
On first run, Relix will ask for your GitLab credentials:
-   **GitLab URL**: e.g., `https://gitlab.com`
-   **Email**: Your GitLab account email.
-   **Token**: A Personal Access Token with `api` scope.

### 2. Project Selection
Select the GitLab project you want to manage. Relix filters projects as you type.
-   Use `Up`/`Down` or `Ctrl+n`/`Ctrl+p` to navigate.
-   Press `Enter` to select.

### 3. Select Merge Requests
Browse the list of Open MRs for the selected project.
-   `Space`: Toggle selection of an MR.
-   `Enter`: Confirm selection and proceed.
-   `o`: Open the highlighted MR in your browser.
-   `r`: Refresh the list.
-   `d`/`u`: Scroll details view down/up.

### 4. Choose Environment
Select the target environment for the release:
-   **DEVELOP** (`develop`)
-   **TEST** (`testing`)
-   **STAGE** (`stable`)
-   **PROD** (`master`)

### 5. Versioning
Enter the semantic version for this release (e.g., `1.2.0`). Relix validates the format (X.Y.Z).

### 6. Release Execution
Relix will perform the git operations step-by-step, showing real-time logs.
-   **Conflict Handling**: If a merge conflict occurs, the process pauses. Resolve the conflict in your terminal, then hit **Retry**.
-   **Abort**: You can abort the process at any time, which attempts to reset your git state to clean.

## ⚙️ Configuration

Relix stores its configuration and state in `~/.relix/`.

-   **`config.json`**: Stores preferences like the last selected project and file exclusion patterns.
-   **`release.json`**: Stores the state of any in-progress release for crash recovery.

### File Exclusions
You can configure files to be automatically removed during the release process (e.g., internal CI configs). This can be edited via the Settings menu (`/` -> Settings) or directly in `config.json`:

```json
{
  "exclude_patterns": ".gitlab-ci.yml\nsprite.gen.ts"
}
```

## ⌨️ Global Shortcuts

-   `/`: Open Command Menu (Settings, Help, etc.)
-   `Ctrl+c`: Quit application.
-   `Esc`: Go back / Close modal.

## 🏗️ Tech Stack

-   **Language**: [Go](https://go.dev/)
-   **TUI Framework**: [Bubble Tea](https://github.com/charmbracelet/bubbletea)
-   **Styling**: [Lipgloss](https://github.com/charmbracelet/lipgloss)
-   **Markdown**: [Glamour](https://github.com/charmbracelet/glamour)
-   **Keyring**: [go-keyring](https://github.com/zalando/go-keyring)

---
*Relix is a tool for developers who want to stay in the flow and automate the tedious parts of release management.*
