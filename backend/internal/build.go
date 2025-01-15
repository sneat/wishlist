package internal

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func (b *Backend) buildFrontend() error {
	start := time.Now()

	b.Logger().Info("Building frontend")

	// Check if the frontend directory exists and is a directory
	if stat, err := os.Stat(b.BuildDir); os.IsNotExist(err) || !stat.IsDir() {
		possiblePaths := []string{"./frontend", "../frontend"}

		// Check if any of the possible paths exist and is a directory
		var validPath string
		for _, path := range possiblePaths {
			if stat, err = os.Stat(path); err == nil && stat.IsDir() {
				validPath = path
				break
			}
		}

		if validPath == "" {
			return errors.New("no valid frontend directory found")
		}

		b.Logger().
			With("old_path", b.BuildDir).
			With("new_path", validPath).
			Info("Updating frontend build directory path")
		b.BuildDir = validPath
	}

	fullPath, err := filepath.Abs(b.BuildDir)
	if err != nil {
		b.Logger().
			With("path", b.BuildDir).
			With("error", err).
			Error("Failed to get absolute path for frontend directory")
		return err
	}

	if b.BuildDir != fullPath {
		b.Logger().
			With("old_path", b.BuildDir).
			With("new_path", fullPath).
			Info("Updating frontend build directory path")
		b.BuildDir = fullPath
	}

	// Run npm install first
	cmd := exec.Command("npm", "install")
	cmd.Dir = b.BuildDir

	if output, err := cmd.CombinedOutput(); err != nil {
		b.Logger().
			With("error", err, "output", string(output)).
			Error("Failed to build frontend (npm install)")
		return err
	}

	cmd = exec.Command("npm", "run", "build")
	cmd.Dir = b.BuildDir

	if output, err := cmd.CombinedOutput(); err != nil {
		b.Logger().
			With("error", err, "output", string(output)).
			Error("Failed to build frontend (npm run build)")
		return err
	}

	b.Logger().
		With("duration", time.Since(start).String()).
		Info("Frontend build completed")

	return nil
}
