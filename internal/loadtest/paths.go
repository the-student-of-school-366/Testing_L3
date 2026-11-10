package loadtest

import (
	"errors"
	"os"
	"path/filepath"
)

// ModuleRoot finds the directory that contains go.mod, starting at the
// process working directory and then at the executable path.
func ModuleRoot() (string, error) {
	if wd, err := os.Getwd(); err == nil {
		if root, err := findModule(wd); err == nil {
			return root, nil
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return "", errors.New("go.mod not found")
	}
	return findModule(filepath.Dir(exe))
}

func findModule(dir string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found")
		}
		dir = parent
	}
}
