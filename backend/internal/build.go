package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// lockfileHash returns the hex-encoded SHA-256 of <buildDir>/package-lock.json.
func lockfileHash(buildDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(buildDir, "package-lock.json"))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (b *Backend) buildFrontend() error {
	start := time.Now()

	b.Logger().Info("Building frontend")

	// Check if the frontend directory exists and is a directory
	if stat, err := os.Stat(b.buildDir); os.IsNotExist(err) || !stat.IsDir() {
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
			With("old_path", b.buildDir).
			With("new_path", validPath).
			Info("Updating frontend build directory path")
		b.buildDir = validPath
	}

	fullPath, err := filepath.Abs(b.buildDir)
	if err != nil {
		b.Logger().
			With("path", b.buildDir).
			With("error", err).
			Error("Failed to get absolute path for frontend directory")
		return err
	}

	if b.buildDir != fullPath {
		b.Logger().
			With("old_path", b.buildDir).
			With("new_path", fullPath).
			Info("Updating frontend build directory path")
		b.buildDir = fullPath
	}

	// Run npm install first
	cmd := exec.Command("npm", "install")
	cmd.Dir = b.buildDir

	if output, err := cmd.CombinedOutput(); err != nil {
		b.Logger().
			With("error", err, "output", string(output)).
			Error("Failed to build frontend (npm install)")
		return err
	}

	cmd = exec.Command("npm", "run", "build")
	cmd.Dir = b.buildDir

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
