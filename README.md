# uvp

`uvp` is a small Go CLI for managing [uv](https://docs.astral.sh/uv/) configuration profiles, inspired by the AWS profile switcher [`asp`](https://github.com/ohmyzsh/ohmyzsh/blob/master/plugins/aws/aws.plugin.zsh).

Each profile is stored as its own file under `~/.config/uv/uv.d/`, and the active config at `~/.config/uv/uv.toml` is a symlink to the selected profile.

## Why profiles?

A common setup is to keep separate profiles for personal and work projects:

- `personal` — your own PyPI index, cache settings, or Python preference
- `work` — corporate index mirror, internal package indexes, or team defaults

Switch between them with one command instead of editing `uv.toml` by hand.

## Installation

### From source

```bash
git clone https://github.com/daniel-soler/uv-profiles.git
cd uv-profiles
go install ./cmd/uvp
```

This installs the `uvp` binary to your `GOBIN` (or `$HOME/go/bin` by default). Add that directory to your `PATH` if `uvp` is not found:

```bash
# zsh (add to ~/.zshrc)
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Build locally

```bash
go build -o uvp ./cmd/uvp
```

## Usage

```bash
uvp                         # Reset to the default profile
uvp <profile>               # Switch to a profile
uvp current                 # Show the active profile
uvp --list                  # List all profiles
uvp -l                      # Same as --list
uvp --create <profile>      # Create a new profile
uvp -c <profile>            # Same as --create
uvp --delete <profile>      # Delete a profile (prompts for confirmation)
uvp -d <profile>            # Same as --delete
uvp --check <profile>       # Validate a profile with uv
```

The active profile is marked with `*` in `--list` output.

`--check` runs `uv` against the profile file to verify TOML syntax and that all settings are allowed in user-level config. Requires `uv` to be installed and on your `PATH`.

### Example: personal and work profiles

```bash
# Start from your current uv.toml (migrated into the default profile on first run)
uvp

# Create profiles seeded from the currently active config
uvp -c personal
uvp -c work

# Edit each profile independently
$EDITOR ~/.config/uv/uv.d/personal.uv.toml
$EDITOR ~/.config/uv/uv.d/work.uv.toml

# Validate before switching
uvp --check work

# Switch as needed
uvp personal
uvp work
uvp --list

# Reset back to default
uvp
```

Example profile files:

`~/.config/uv/uv.d/personal.uv.toml`

```toml
index-url = "https://pypi.org/simple"
```

`~/.config/uv/uv.d/work.uv.toml`

```toml
index-url = "https://pypi.company.internal/simple"
extra-index-url = ["https://artifacts.company.internal/simple"]
```

## How it works

On first run, `uvp`:

1. Creates `~/.config/uv/uv.d/` if needed
2. Creates `~/.config/uv/uv.d/default.uv.toml` if missing
3. Migrates an existing plain `~/.config/uv/uv.toml` file into the default profile
4. Replaces `~/.config/uv/uv.toml` with a symlink to the active profile

Profile files use the naming pattern:

```text
~/.config/uv/uv.d/<profile>.uv.toml
```

Switching profiles updates the `uv.toml` symlink only; profile files are never modified unless you edit or delete them.

## Development

Requirements:

- Go 1.22+

Common commands:

```bash
# Run tests
go test ./...

# Build the binary
go build -o uvp ./cmd/uvp

# Run locally without installing
go run ./cmd/uvp --list
```

Project layout:

```text
.
├── cmd/uvp/       # main entrypoint (binary name: uvp)
├── internal/
│   ├── cli/       # argument parsing and command handlers
│   └── config/    # profile storage and symlink management
└── README.md
```

## Notes

- Profile names may contain letters, numbers, hyphens, and underscores.
- The `default` profile cannot be deleted.
- Deleting the active profile switches back to `default`.
- New profiles are seeded from the currently active config.

## License

MIT
