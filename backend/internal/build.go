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

const lockfileHashMarker = ".wishlist-lockfile-hash"

type buildMode int

const (
	buildModeCached buildMode = iota
	buildModeForceReinstall
)

func lockfileHash(buildDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(buildDir, "package-lock.json"))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func shouldInstallDeps(buildDir string, mode buildMode) (bool, string) {
	if mode == buildModeForceReinstall {
		return true, "force reinstall"
	}

	nmInfo, err := os.Stat(filepath.Join(buildDir, "node_modules"))
	if err != nil || !nmInfo.IsDir() {
		return true, "node_modules missing"
	}

	markerBytes, err := os.ReadFile(filepath.Join(buildDir, "node_modules", lockfileHashMarker))
	if err != nil {
		return true, "marker missing"
	}

	current, err := lockfileHash(buildDir)
	if err != nil {
		return true, "lockfile unreadable"
	}

	if string(markerBytes) != current {
		return true, "lockfile changed"
	}

	return false, "lockfile unchanged"
}

func (b *Backend) buildFrontend(mode buildMode) error {
	start := time.Now()

	b.Logger().Info("Building frontend")

	if err := b.resolveBuildDir(); err != nil {
		return err
	}

	if err := b.ensureDependencies(mode); err != nil {
		return err
	}

	if err := b.runAstroBuild(); err != nil {
		return err
	}

	b.Logger().
		With("duration", time.Since(start).String()).
		Info("Frontend build completed")

	return nil
}

func (b *Backend) resolveBuildDir() error {
	if stat, err := os.Stat(b.buildDir); os.IsNotExist(err) || (err == nil && !stat.IsDir()) {
		possiblePaths := []string{"./frontend", "../frontend"}

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

	return nil
}

func (b *Backend) ensureDependencies(mode buildMode) error {
	_ = mode

	cmd := exec.Command("npm", "install")
	cmd.Dir = b.buildDir

	if output, err := cmd.CombinedOutput(); err != nil {
		b.Logger().
			With("error", err, "output", string(output)).
			Error("Failed to build frontend (npm install)")
		return err
	}

	return nil
}

func (b *Backend) runAstroBuild() error {
	cmd := exec.Command("npm", "run", "build")
	cmd.Dir = b.buildDir

	if output, err := cmd.CombinedOutput(); err != nil {
		b.Logger().
			With("error", err, "output", string(output)).
			Error("Failed to build frontend (npm run build)")
		return err
	}

	return nil
}
