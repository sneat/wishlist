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
