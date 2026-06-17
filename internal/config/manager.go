package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	ErrProfileExists    = errors.New("profile already exists")
	ErrProfileNotFound  = errors.New("profile not found")
	ErrInvalidName      = errors.New("invalid profile name")
	ErrDeleteCancelled  = errors.New("delete cancelled")
)

// Manager handles uv profile storage and activation.
type Manager struct {
	paths Paths
}

// NewManager creates a manager using the default uv config paths.
func NewManager() (*Manager, error) {
	paths, err := DefaultPaths()
	if err != nil {
		return nil, err
	}
	return &Manager{paths: paths}, nil
}

// NewManagerWithPaths creates a manager with custom paths (mainly for tests).
func NewManagerWithPaths(paths Paths) *Manager {
	return &Manager{paths: paths}
}

// EnsureLayout creates required directories and migrates an existing uv.toml file
// into the default profile when needed.
func (m *Manager) EnsureLayout() error {
	if err := os.MkdirAll(m.paths.ProfilesDir, 0o755); err != nil {
		return fmt.Errorf("create profiles directory: %w", err)
	}

	defaultPath := m.paths.ProfilePath(DefaultProfileName)
	if _, err := os.Stat(defaultPath); errors.Is(err, fs.ErrNotExist) {
		if err := m.bootstrapDefaultProfile(defaultPath); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("stat default profile: %w", err)
	}

	return m.ensureActiveSymlink(defaultPath)
}

func (m *Manager) bootstrapDefaultProfile(defaultPath string) error {
	activeInfo, err := os.Lstat(m.paths.ActiveFile)
	if err == nil {
		if activeInfo.Mode()&os.ModeSymlink != 0 {
			target, readErr := os.Readlink(m.paths.ActiveFile)
			if readErr != nil {
				return fmt.Errorf("read active config symlink: %w", readErr)
			}
			if filepath.Clean(target) == filepath.Clean(defaultPath) {
				return os.WriteFile(defaultPath, []byte("# uv default profile\n"), 0o644)
			}
		}

		content, readErr := os.ReadFile(m.paths.ActiveFile)
		if readErr != nil {
			return fmt.Errorf("read existing uv.toml: %w", readErr)
		}
		if err := os.WriteFile(defaultPath, content, 0o644); err != nil {
			return fmt.Errorf("write default profile: %w", err)
		}
		return m.setActiveProfile(DefaultProfileName)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stat active config: %w", err)
	}

	return os.WriteFile(defaultPath, []byte("# uv default profile\n"), 0o644)
}

func (m *Manager) ensureActiveSymlink(defaultPath string) error {
	info, err := os.Lstat(m.paths.ActiveFile)
	if errors.Is(err, fs.ErrNotExist) {
		return m.createSymlink(defaultPath)
	}
	if err != nil {
		return fmt.Errorf("stat active config: %w", err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		content, readErr := os.ReadFile(m.paths.ActiveFile)
		if readErr != nil {
			return fmt.Errorf("read active config: %w", readErr)
		}
		if err := os.WriteFile(defaultPath, content, 0o644); err != nil {
			return fmt.Errorf("write default profile: %w", err)
		}
		if err := os.Remove(m.paths.ActiveFile); err != nil {
			return fmt.Errorf("remove active config file: %w", err)
		}
		return m.createSymlink(defaultPath)
	}

	return nil
}

// ActiveProfile returns the currently active profile name.
func (m *Manager) ActiveProfile() (string, error) {
	target, err := os.Readlink(m.paths.ActiveFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read active config symlink: %w", err)
	}

	resolved := target
	if !filepath.IsAbs(target) {
		resolved = filepath.Join(m.paths.ConfigDir, target)
	}

	name, err := ProfileNameFromPath(resolved)
	if err != nil {
		return "", err
	}
	return name, nil
}

// ListProfiles returns all available profile names sorted alphabetically.
func (m *Manager) ListProfiles() ([]string, error) {
	entries, err := os.ReadDir(m.paths.ProfilesDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read profiles directory: %w", err)
	}

	var profiles []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ProfileSuffix) {
			continue
		}
		name, nameErr := ProfileNameFromPath(entry.Name())
		if nameErr != nil {
			continue
		}
		profiles = append(profiles, name)
	}
	sort.Strings(profiles)
	return profiles, nil
}

// UseProfile activates the named profile by updating the uv.toml symlink.
func (m *Manager) UseProfile(name string) error {
	if err := validateProfileName(name); err != nil {
		return err
	}

	profilePath := m.paths.ProfilePath(name)
	if _, err := os.Stat(profilePath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: %q", ErrProfileNotFound, name)
		}
		return fmt.Errorf("stat profile: %w", err)
	}

	return m.setActiveProfile(name)
}

// ResetDefault activates the default profile.
func (m *Manager) ResetDefault() error {
	return m.UseProfile(DefaultProfileName)
}

// CreateProfile creates a new profile, optionally seeded from the active config.
func (m *Manager) CreateProfile(name string) error {
	if err := validateProfileName(name); err != nil {
		return err
	}

	profilePath := m.paths.ProfilePath(name)
	if _, err := os.Stat(profilePath); err == nil {
		return fmt.Errorf("%w: %q", ErrProfileExists, name)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stat profile: %w", err)
	}

	content, err := m.readActiveConfigContent()
	if err != nil {
		return err
	}
	if len(content) == 0 {
		content = []byte(fmt.Sprintf("# uv profile: %s\n", name))
	}

	if err := os.WriteFile(profilePath, content, 0o644); err != nil {
		return fmt.Errorf("create profile: %w", err)
	}
	return nil
}

// CheckProfile validates the named profile using uv.
func (m *Manager) CheckProfile(name string, validate ConfigValidator) error {
	if validate == nil {
		validate = ValidateWithUV
	}
	if err := validateProfileName(name); err != nil {
		return err
	}

	profilePath := m.paths.ProfilePath(name)
	if _, err := os.Stat(profilePath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: %q", ErrProfileNotFound, name)
		}
		return fmt.Errorf("stat profile: %w", err)
	}

	return validate(profilePath)
}

// DeleteProfile removes a profile file after optional confirmation.
func (m *Manager) DeleteProfile(name string, confirm func(string) (bool, error)) error {
	if err := validateProfileName(name); err != nil {
		return err
	}
	if name == DefaultProfileName {
		return fmt.Errorf("cannot delete the default profile")
	}

	profilePath := m.paths.ProfilePath(name)
	if _, err := os.Stat(profilePath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: %q", ErrProfileNotFound, name)
		}
		return fmt.Errorf("stat profile: %w", err)
	}

	ok, err := confirm(name)
	if err != nil {
		return err
	}
	if !ok {
		return ErrDeleteCancelled
	}

	active, err := m.ActiveProfile()
	if err != nil {
		return err
	}

	if err := os.Remove(profilePath); err != nil {
		return fmt.Errorf("delete profile: %w", err)
	}

	if active == name {
		return m.ResetDefault()
	}
	return nil
}

func (m *Manager) readActiveConfigContent() ([]byte, error) {
	info, err := os.Lstat(m.paths.ActiveFile)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("stat active config: %w", err)
	}

	path := m.paths.ActiveFile
	if info.Mode()&os.ModeSymlink != 0 {
		target, readErr := os.Readlink(m.paths.ActiveFile)
		if readErr != nil {
			return nil, fmt.Errorf("read active config symlink: %w", readErr)
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(m.paths.ConfigDir, target)
		}
		path = target
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read active config: %w", err)
	}
	return content, nil
}

func (m *Manager) setActiveProfile(name string) error {
	return m.createSymlink(m.paths.ProfilePath(name))
}

func (m *Manager) createSymlink(target string) error {
	if err := os.Remove(m.paths.ActiveFile); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove active config: %w", err)
	}

	relTarget, err := filepath.Rel(filepath.Dir(m.paths.ActiveFile), target)
	if err != nil {
		relTarget = target
	}

	if err := os.Symlink(relTarget, m.paths.ActiveFile); err != nil {
		return fmt.Errorf("create active config symlink: %w", err)
	}
	return nil
}

func validateProfileName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: name cannot be empty", ErrInvalidName)
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return fmt.Errorf("%w: %q may only contain letters, numbers, hyphens, and underscores", ErrInvalidName, name)
		}
	}
	return nil
}

// ConfirmDelete prompts on stdin before deleting a profile.
func ConfirmDelete(in io.Reader, out io.Writer, name string) (bool, error) {
	fmt.Fprintf(out, "Delete profile %q? [y/N] ", name)
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read confirmation: %w", err)
	}

	answer := strings.TrimSpace(strings.ToLower(line))
	return answer == "y" || answer == "yes", nil
}
