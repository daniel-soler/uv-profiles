package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/daniel/uv-profiles/internal/cli"
)

func TestRunListAndSwitch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var stdout, stderr bytes.Buffer
	if code := cli.Run([]string{"uvp", "--create", "work"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("create exit code = %d, stderr = %s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"uvp", "--list"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("list exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "default") || !strings.Contains(stdout.String(), "work") {
		t.Fatalf("list output = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"uvp", "work"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("switch exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `Switched to profile "work"`) {
		t.Fatalf("switch output = %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"uvp", "current"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("current exit code = %d, stderr = %s", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "work" {
		t.Fatalf("current output = %q, want work", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"uvp"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("reset exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "default profile") {
		t.Fatalf("reset output = %q", stdout.String())
	}
}

func TestRunCheck(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var stdout, stderr bytes.Buffer
	if code := cli.Run([]string{"uvp", "--check", "default"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		if strings.Contains(stderr.String(), "uv not found") {
			t.Skip("uv not installed")
		}
		t.Fatalf("check exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `Profile "default" is valid`) {
		t.Fatalf("check output = %q", stdout.String())
	}
}

func TestRunDeleteCancelled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var stdout, stderr bytes.Buffer
	if code := cli.Run([]string{"uvp", "-c", "personal"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("create exit code = %d, stderr = %s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"uvp", "-d", "personal"}, strings.NewReader("n\n"), &stdout, &stderr); code != 0 {
		t.Fatalf("delete exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Delete cancelled.") {
		t.Fatalf("delete output = %q", stdout.String())
	}
}
