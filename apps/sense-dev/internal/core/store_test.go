package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenRejectsNonPrivateStateRoot(t *testing.T) {
	for _, mode := range []os.FileMode{0755, 0710, 0704} {
		root := filepath.Join(t.TempDir(), "state")
		if err := os.Mkdir(root, mode); err != nil {
			t.Fatal(err)
		}
		// The process umask can only make the directory more restrictive.
		if err := os.Chmod(root, mode); err != nil {
			t.Fatal(err)
		}
		if store, err := Open(root); err == nil {
			_ = store.Close()
			t.Fatalf("accepted state directory mode %04o", mode)
		}
		if _, err := os.Stat(filepath.Join(root, "controller.lock")); !os.IsNotExist(err) {
			t.Fatalf("created a lock in rejected state directory mode %04o: %v", mode, err)
		}
	}
}

func TestOpenRejectsSymlinkAndFilesystemRoot(t *testing.T) {
	parent := t.TempDir()
	actual := filepath.Join(parent, "actual")
	if err := os.Mkdir(actual, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "state-link")
	if err := os.Symlink(actual, link); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{link, link + string(filepath.Separator), "/", "/tmp/.."} {
		if store, err := Open(root); err == nil {
			_ = store.Close()
			t.Fatalf("accepted unsafe state root %q", root)
		}
	}
	if _, err := os.Stat(filepath.Join(actual, "controller.lock")); !os.IsNotExist(err) {
		t.Fatalf("created a lock through rejected symlink: %v", err)
	}
}
