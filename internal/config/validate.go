package config

import (
	"fmt"
	"os/exec"
	"strings"
)

// ConfigValidator validates a uv.toml file.
type ConfigValidator func(path string) error

// ValidateWithUV checks a profile file by running uv with --config-file.
func ValidateWithUV(path string) error {
	uvBin, err := exec.LookPath("uv")
	if err != nil {
		return fmt.Errorf("uv not found in PATH: install uv or add it to PATH")
	}

	cmd := exec.Command(uvBin, "pip", "list", "--config-file", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			return fmt.Errorf("uv config validation failed")
		}
		return fmt.Errorf("%s", message)
	}
	return nil
}
