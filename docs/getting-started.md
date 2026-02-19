[Back to README](../README.md) | [Usage Guide ->](usage.md)

# Getting Started

## Prerequisites

- **Go** 1.25 or higher
- **Git** installed and available in your PATH
- **GitLab account** with a Personal Access Token (PAT) with `api` scope

## Installation

### Build from Source

```bash
git clone https://github.com/yourusername/relix.git
cd relix
go build -o relix .
```

### Install Directly

```bash
go install github.com/yourusername/relix@latest
```

## First Run

Launch Relix from the root of any Git project:

```bash
./relix
```

Or specify a project directory:

```bash
./relix -d /path/to/your/project
```

### CLI Flags

| Flag | Description |
|------|-------------|
| `-d`, `--project-directory` | Project root directory path (default: current dir) |
| `-h`, `--help` | Show help message |
| `-v`, `--version` | Show version |

## Authentication

On first run, Relix prompts for your GitLab credentials:

1. **GitLab URL** -- your GitLab instance (e.g., `https://gitlab.com`)
2. **Email** -- your GitLab account email
3. **Token** -- a Personal Access Token with `api` scope

Credentials are stored securely in your system's keyring (macOS Keychain, Windows Credential Manager, or Linux Secret Service). They persist across sessions and are never stored in plain text.

To create a PAT in GitLab:
1. Go to **Settings > Access Tokens** in your GitLab instance
2. Create a token with the `api` scope
3. Copy the token and paste it into Relix

## Next Steps

After authentication, you'll land on the Home screen where you can:

- **Select a project** to manage
- **Create a new release** by selecting MRs
- **View release history** for past releases

Continue to the [Usage Guide](usage.md) for the full workflow walkthrough.

## See Also

- [Usage Guide](usage.md) -- full release workflow walkthrough
- [Configuration](configuration.md) -- customize environments, themes, and exclusions
