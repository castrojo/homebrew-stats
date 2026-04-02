package ghcli

import (
	"fmt"
	"os/exec"
)

// Run executes the gh CLI with the given arguments and returns stdout.
// gh reads GITHUB_TOKEN from the environment automatically.
func Run(args ...string) ([]byte, error) {
	out, err := exec.Command("gh", args...).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("gh %v: %w\nstderr: %s", args, err, exitErr.Stderr)
		}
		return nil, fmt.Errorf("gh %v: %w", args, err)
	}
	return out, nil
}
