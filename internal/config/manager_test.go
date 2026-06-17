package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/daniel/uv-profiles/internal/config"
)

func testPaths(t *testing.T) config.Paths {
	t.Helper()

	root := t.TempDir()
	configDir := filepath.Join(root, ".config", "uv")
	return config.Paths{
		ConfigDir:   configDir,
		ProfilesDir: filepath.Join(configDir, "uv.d"),
		ActiveFile:  filepath.Join(configDir, "uv.toml"),
	}
}

func TestEnsureLayoutMigratesExistingConfig(t *testing.T) {
	paths := testPaths(t)
	if err := os.MkdirAll(paths.ConfigDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.ActiveFile, []byte("index-url = \"https://example.com/simple\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manager := config.NewManagerWithPaths(paths)
	if err := manager.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}

	defaultPath := paths.ProfilePath(config.DefaultProfileName)
	content, err := os.ReadFile(defaultPath)
	if err != nil {
		t.Fatalf("ReadFile(default) error = %v", err)
	}
	if !strings.Contains(string(content), "index-url") {
		t.Fatalf("default profile content = %q, want migrated config", content)
	}

	info, err := os.Lstat(paths.ActiveFile)
	if err != nil {
		t.Fatalf("Lstat(active) error = %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("active config is not a symlink after migration")
	}
}

func TestCreateListUseAndReset(t *testing.T) {
	paths := testPaths(t)
	manager := config.NewManagerWithPaths(paths)
	if err := manager.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}

	if err := manager.CreateProfile("work"); err != nil {
		t.Fatalf("CreateProfile(work) error = %v", err)
	}

	profiles, err := manager.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles() error = %v", err)
	}
	if len(profiles) != 2 || profiles[0] != config.DefaultProfileName || profiles[1] != "work" {
		t.Fatalf("ListProfiles() = %v, want [default work]", profiles)
	}

	if err := manager.UseProfile("work"); err != nil {
		t.Fatalf("UseProfile(work) error = %v", err)
	}

	active, err := manager.ActiveProfile()
	if err != nil {
		t.Fatalf("ActiveProfile() error = %v", err)
	}
	if active != "work" {
		t.Fatalf("ActiveProfile() = %q, want work", active)
	}

	if err := manager.ResetDefault(); err != nil {
		t.Fatalf("ResetDefault() error = %v", err)
	}

	active, err = manager.ActiveProfile()
	if err != nil {
		t.Fatalf("ActiveProfile() error = %v", err)
	}
	if active != config.DefaultProfileName {
		t.Fatalf("ActiveProfile() = %q, want default", active)
	}
}

func TestDeleteProfileRequiresConfirmation(t *testing.T) {
	paths := testPaths(t)
	manager := config.NewManagerWithPaths(paths)
	if err := manager.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}
	if err := manager.CreateProfile("personal"); err != nil {
		t.Fatalf("CreateProfile(personal) error = %v", err)
	}

	err := manager.DeleteProfile("personal", func(string) (bool, error) {
		return false, nil
	})
	if err != config.ErrDeleteCancelled {
		t.Fatalf("DeleteProfile() error = %v, want ErrDeleteCancelled", err)
	}

	if _, err := os.Stat(paths.ProfilePath("personal")); err != nil {
		t.Fatalf("profile file should still exist: %v", err)
	}
}

func TestDeleteActiveProfileResetsDefault(t *testing.T) {
	paths := testPaths(t)
	manager := config.NewManagerWithPaths(paths)
	if err := manager.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}
	if err := manager.CreateProfile("work"); err != nil {
		t.Fatalf("CreateProfile(work) error = %v", err)
	}
	if err := manager.UseProfile("work"); err != nil {
		t.Fatalf("UseProfile(work) error = %v", err)
	}

	if err := manager.DeleteProfile("work", func(string) (bool, error) {
		return true, nil
	}); err != nil {
		t.Fatalf("DeleteProfile(work) error = %v", err)
	}

	active, err := manager.ActiveProfile()
	if err != nil {
		t.Fatalf("ActiveProfile() error = %v", err)
	}
	if active != config.DefaultProfileName {
		t.Fatalf("ActiveProfile() = %q, want default", active)
	}
}

func TestCheckProfile(t *testing.T) {
	paths := testPaths(t)
	manager := config.NewManagerWithPaths(paths)
	if err := manager.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}

	err := manager.CheckProfile("missing", func(string) error { return nil })
	if !errors.Is(err, config.ErrProfileNotFound) {
		t.Fatalf("CheckProfile(missing) error = %v, want ErrProfileNotFound", err)
	}

	called := false
	err = manager.CheckProfile("default", func(path string) error {
		called = true
		if path != paths.ProfilePath("default") {
			t.Fatalf("validate path = %q, want %q", path, paths.ProfilePath("default"))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("CheckProfile(default) error = %v", err)
	}
	if !called {
		t.Fatal("expected validator to be called")
	}
}

func TestValidateProfileName(t *testing.T) {
	paths := testPaths(t)
	manager := config.NewManagerWithPaths(paths)
	if err := manager.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout() error = %v", err)
	}

	if err := manager.CreateProfile("bad name"); err == nil {
		t.Fatal("CreateProfile(bad name) expected error")
	}
}
