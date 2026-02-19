[<- Usage Guide](usage.md) | [Back to README](../README.md) | [Architecture ->](architecture.md)

# Configuration

## Config File

Relix stores configuration in `~/.relix/config.json`. Settings can be edited via the Settings modal (`/` -> Settings) or directly in the file.

### Config Structure

```json
{
  "selected_project_id": 12345,
  "selected_project_path": "namespace/project",
  "selected_project_name": "Namespace / Project Name",
  "selected_project_short_name": "project",
  "base_branch": "root",
  "environments": [
    {"name": "develop", "branch_name": "develop"},
    {"name": "test", "branch_name": "testing"},
    {"name": "stage", "branch_name": "stable"},
    {"name": "prod", "branch_name": "master"}
  ],
  "exclude_patterns": ".gitlab-ci.yml\nsprite.gen.ts",
  "selected_theme": "indigo",
  "themes": [...]
}
```

## Environments

Four environments are configurable, each with a display name and git branch:

| Setting | Default Name | Default Branch |
|---------|-------------|----------------|
| Environment 1 | DEVELOP | `develop` |
| Environment 2 | TEST | `testing` |
| Environment 3 | STAGE | `stable` |
| Environment 4 | PROD | `master` |

Both display names and branch mappings are editable in Settings. Display names are shown in UPPERCASE in the UI.

## Base Branch

The base branch (default: `root`) is the branch from which releases are created and optionally merged back to. Configure via Settings or `"base_branch"` in config.

## File Exclusions

Define file patterns to automatically exclude from release builds. One pattern per line.

```
.gitlab-ci.yml
sprite.gen.ts
```

**Supported patterns:**
- `*` -- match any characters in filename
- `**/` -- match any directory depth

**Validation rules:**
- No empty lines
- Max 80 characters per line
- No invalid characters: `<>:"|?\`
- No overly broad patterns (`*`, `**`, `/`)

## Themes

Relix supports custom themes with full color control. The default theme is `indigo`.

### Adding Custom Themes

Themes must be added directly to `~/.relix/config.json` in the `"themes"` array. The Settings UI allows selecting from existing themes but not creating new ones.

### Theme Colors

**Required fields:**

| Field | Description |
|-------|-------------|
| `name` | Unique theme identifier |
| `accent` | Primary accent color (hex) |
| `foreground` | Main text color (hex) |
| `notion` | Subtle/secondary color (hex) |
| `success` | Success state color (hex) |
| `warning` | Warning state color (hex) |
| `error` | Error state color (hex) |

**Optional fields (auto-derived if omitted):**

| Field | Fallback |
|-------|----------|
| `accent_foreground` | High-contrast against accent |
| `notion_foreground` | Same as foreground |
| `success_foreground` | Black or white (contrast) |
| `warning_foreground` | Black or white (contrast) |
| `error_foreground` | Black or white (contrast) |
| `muted` | Derived from accent |
| `muted_foreground` | Derived from foreground |
| `env_develop` | Same as success |
| `env_test` | Same as warning |
| `env_stage` | Same as error |
| `env_prod` | Same as accent |

### Example Theme

```json
{
  "name": "ocean",
  "accent": "#0077B6",
  "foreground": "#CAF0F8",
  "notion": "#023E8A",
  "success": "#06D6A0",
  "warning": "#FFD166",
  "error": "#EF476F"
}
```

## Credentials

GitLab credentials are stored in your system's keyring (macOS Keychain, Windows Credential Manager, Linux Secret Service). They include:

- GitLab instance URL
- Account email
- Personal Access Token

Credentials can be updated by logging out and re-authenticating.

## Storage Locations

| Path | Purpose |
|------|---------|
| `~/.relix/config.json` | User preferences, project, themes |
| `~/.relix/release.json` | In-progress release state (deleted on completion) |
| `~/.local/.relix/releases/index.json` | Release history index |
| `~/.local/.relix/releases/{timestamp}.json` | Individual release details |
| System keyring | GitLab credentials |

## See Also

- [Usage Guide](usage.md) -- how environments and exclusions affect the release workflow
- [Architecture](architecture.md) -- how configuration is loaded and persisted
