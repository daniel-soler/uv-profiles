package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultProfileName = "default"
	ProfileSuffix      = ".uv.toml"
)

// Paths holds resolved uv configuration locations.
type Paths struct {
	ConfigDir   string
	ProfilesDir string
	ActiveFile  string
}

// DefaultPaths returns the standard uv config locations under the user home directory.
func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve home directory: %w", err)
	}

	configDir := filepath.Join(home, ".config", "uv")
	return Paths{
		ConfigDir:   configDir,
		ProfilesDir: filepath.Join(configDir, "uv.d"),
		ActiveFile:  filepath.Join(configDir, "uv.toml"),
	}, nil
}

// ProfilePath returns the on-disk path for a named profile.
func (p Paths) ProfilePath(name string) string {
	return filepath.Join(p.ProfilesDir, name+ProfileSuffix)
}

// ProfileNameFromPath extracts the profile name from a profile file path.
func ProfileNameFromPath(path string) (string, error) {
	base := filepath.Base(path)
	if !strings.HasSuffix(base, ProfileSuffix) {
		return "", fmt.Errorf("invalid profile file %q", path)
	}
	return strings.TrimSuffix(base, ProfileSuffix), nil
}
