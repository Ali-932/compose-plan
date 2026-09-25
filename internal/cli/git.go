package cli

import (
	"os/exec"
	"strings"
)

// gitHead returns the short commit checked out in dir, or "" if dir is not a git repository.
func gitHead(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
