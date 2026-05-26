package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLockfileHash(t *testing.T) {
	t.Run("hashes lockfile contents deterministically", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte(`{"name":"x"}`), 0o644); err != nil {
			t.Fatal(err)
		}

		got, err := lockfileHash(dir)
		if err != nil {
			t.Fatalf("lockfileHash: %v", err)
		}

		// SHA-256 of `{"name":"x"}` — verified with `printf '{"name":"x"}' | shasum -a 256`.
		want := "0229d37e33daae149bf40543a5ce1db4459d10f830d5139279aa2bfd5f6485a1"
		if got != want {
			t.Fatalf("hash mismatch: got %q want %q", got, want)
		}
	})

	t.Run("returns error when lockfile missing", func(t *testing.T) {
		dir := t.TempDir()
		if _, err := lockfileHash(dir); err == nil {
			t.Fatal("expected error for missing lockfile, got nil")
		}
	})
}

func TestShouldInstallDeps(t *testing.T) {
	const lockfileContents = `{"name":"x"}`
	const lockfileSHA = "0229d37e33daae149bf40543a5ce1db4459d10f830d5139279aa2bfd5f6485a1"
	const otherSHA = "0000000000000000000000000000000000000000000000000000000000000000"

	// nodeModulesKind controls how <buildDir>/node_modules is set up:
	//   "" — absent; "dir" — a directory; "file" — a regular file (degenerate case).
	type fixture struct {
		name             string
		writeLockfile    bool
		nodeModulesKind  string
		writeMarker      bool
		markerContents   string
		force            bool
		wantInstall      bool
	}

	cases := []fixture{
		{
			name:          "marker missing forces install",
			writeLockfile: true, nodeModulesKind: "dir", writeMarker: false,
			wantInstall: true,
		},
		{
			name:          "marker matches lockfile and node_modules exists skips install",
			writeLockfile: true, nodeModulesKind: "dir", writeMarker: true, markerContents: lockfileSHA,
			wantInstall: false,
		},
		{
			name:          "stale marker forces install",
			writeLockfile: true, nodeModulesKind: "dir", writeMarker: true, markerContents: otherSHA,
			wantInstall: true,
		},
		{
			name:          "node_modules is a file not a directory forces install",
			writeLockfile: true, nodeModulesKind: "file", writeMarker: false,
			wantInstall: true,
		},
		{
			name:          "force=true installs even when marker matches",
			writeLockfile: true, nodeModulesKind: "dir", writeMarker: true, markerContents: lockfileSHA,
			force:         true,
			wantInstall:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()

			if tc.writeLockfile {
				if err := os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte(lockfileContents), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			switch tc.nodeModulesKind {
			case "dir":
				if err := os.MkdirAll(filepath.Join(dir, "node_modules"), 0o755); err != nil {
					t.Fatal(err)
				}
			case "file":
				if err := os.WriteFile(filepath.Join(dir, "node_modules"), []byte("not a directory"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tc.writeMarker {
				if err := os.WriteFile(filepath.Join(dir, "node_modules", lockfileHashMarker), []byte(tc.markerContents), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			got, reason := shouldInstallDeps(dir, tc.force)
			if got != tc.wantInstall {
				t.Fatalf("shouldInstallDeps: got (%v, %q), want install=%v", got, reason, tc.wantInstall)
			}
			if got && reason == "" {
				t.Fatalf("install required but reason is empty")
			}
		})
	}
}
