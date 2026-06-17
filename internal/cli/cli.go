package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/daniel/uv-profiles/internal/config"
)

// Run executes the uvp CLI with the provided args and I/O streams.
func Run(args []string, in io.Reader, out io.Writer, errOut io.Writer) int {
	if len(args) == 0 {
		args = []string{"uvp"}
	}

	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(errOut)

	list := fs.Bool("list", false, "list available profiles")
	listShort := fs.Bool("l", false, "list available profiles (shorthand)")
	create := fs.String("create", "", "create a new profile")
	createShort := fs.String("c", "", "create a new profile (shorthand)")
	deleteProfile := fs.String("delete", "", "delete a profile")
	deleteShort := fs.String("d", "", "delete a profile (shorthand)")
	check := fs.String("check", "", "validate a profile")

	fs.Usage = func() {
		fmt.Fprintln(out, "Manage uv configuration profiles.")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Usage:")
		fmt.Fprintln(out, "  uvp                         Reset to the default profile")
		fmt.Fprintln(out, "  uvp <profile>               Switch to a profile")
		fmt.Fprintln(out, "  uvp current                 Show the active profile")
		fmt.Fprintln(out, "  uvp --list, -l              List all profiles")
		fmt.Fprintln(out, "  uvp --create, -c <profile>  Create a new profile")
		fmt.Fprintln(out, "  uvp --delete, -d <profile>  Delete a profile")
		fmt.Fprintln(out, "  uvp --check <profile>       Validate a profile with uv")
	}

	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	manager, err := config.NewManager()
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}
	if err := manager.EnsureLayout(); err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}

	remaining := fs.Args()

	switch {
	case *list || *listShort:
		return runList(manager, out, errOut)
	case *create != "" || *createShort != "":
		name := coalesce(*create, *createShort)
		if len(remaining) > 0 {
			name = remaining[0]
		}
		return runCreate(manager, name, out, errOut)
	case *deleteProfile != "" || *deleteShort != "":
		name := coalesce(*deleteProfile, *deleteShort)
		if len(remaining) > 0 {
			name = remaining[0]
		}
		return runDelete(manager, name, in, out, errOut)
	case *check != "":
		name := *check
		if len(remaining) > 0 {
			name = remaining[0]
		}
		return runCheck(manager, name, out, errOut)
	case len(remaining) == 0:
		return runReset(manager, out, errOut)
	case len(remaining) == 1 && remaining[0] == "current":
		return runCurrent(manager, out, errOut)
	case len(remaining) == 1:
		return runUse(manager, remaining[0], out, errOut)
	default:
		fs.Usage()
		return 2
	}
}

func runList(manager *config.Manager, out io.Writer, errOut io.Writer) int {
	profiles, err := manager.ListProfiles()
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}

	active, err := manager.ActiveProfile()
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}

	if len(profiles) == 0 {
		fmt.Fprintln(out, "No profiles found.")
		return 0
	}

	for _, profile := range profiles {
		if profile == active {
			fmt.Fprintf(out, "* %s\n", profile)
			continue
		}
		fmt.Fprintf(out, "  %s\n", profile)
	}
	return 0
}

func runCreate(manager *config.Manager, name string, out io.Writer, errOut io.Writer) int {
	if strings.TrimSpace(name) == "" {
		fmt.Fprintln(errOut, "error: profile name is required")
		return 2
	}

	if err := manager.CreateProfile(name); err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return exitCodeForError(err)
	}

	fmt.Fprintf(out, "Created profile %q at ~/.config/uv/uv.d/%s.uv.toml\n", name, name)
	return 0
}

func runDelete(manager *config.Manager, name string, in io.Reader, out io.Writer, errOut io.Writer) int {
	if strings.TrimSpace(name) == "" {
		fmt.Fprintln(errOut, "error: profile name is required")
		return 2
	}

	err := manager.DeleteProfile(name, func(profileName string) (bool, error) {
		return config.ConfirmDelete(in, out, profileName)
	})
	if err != nil {
		if errors.Is(err, config.ErrDeleteCancelled) {
			fmt.Fprintln(out, "Delete cancelled.")
			return 0
		}
		fmt.Fprintf(errOut, "error: %v\n", err)
		return exitCodeForError(err)
	}

	fmt.Fprintf(out, "Deleted profile %q\n", name)
	return 0
}

func runReset(manager *config.Manager, out io.Writer, errOut io.Writer) int {
	if err := manager.ResetDefault(); err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(out, "Switched to default profile\n")
	return 0
}

func runCheck(manager *config.Manager, name string, out io.Writer, errOut io.Writer) int {
	if strings.TrimSpace(name) == "" {
		fmt.Fprintln(errOut, "error: profile name is required")
		return 2
	}

	if err := manager.CheckProfile(name, nil); err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return exitCodeForError(err)
	}

	fmt.Fprintf(out, "Profile %q is valid\n", name)
	return 0
}

func runCurrent(manager *config.Manager, out io.Writer, errOut io.Writer) int {
	active, err := manager.ActiveProfile()
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return 1
	}
	if active == "" {
		fmt.Fprintln(errOut, "error: no active profile")
		return 1
	}

	fmt.Fprintln(out, active)
	return 0
}

func runUse(manager *config.Manager, name string, out io.Writer, errOut io.Writer) int {
	if err := manager.UseProfile(name); err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err)
		return exitCodeForError(err)
	}
	fmt.Fprintf(out, "Switched to profile %q\n", name)
	return 0
}

func exitCodeForError(err error) int {
	switch {
	case errors.Is(err, config.ErrProfileNotFound):
		return 1
	case errors.Is(err, config.ErrProfileExists):
		return 1
	case errors.Is(err, config.ErrInvalidName):
		return 2
	default:
		return 1
	}
}

func coalesce(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// Main is the real program entrypoint wrapper.
func Main() {
	os.Exit(Run(os.Args, os.Stdin, os.Stdout, os.Stderr))
}
