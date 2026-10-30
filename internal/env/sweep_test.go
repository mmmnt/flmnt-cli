package env

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoSourceOutsideThisPackageNamesAQuorumVariable sweeps every non-test Go file in the module: a
// QUORUM_ name anywhere else is a second resolver, or help text naming the former variable.
func TestNoSourceOutsideThisPackageNamesAQuorumVariable(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	here, _ := filepath.Abs(".")
	scanned := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "dist", ".feature":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || filepath.Dir(path) == here {
			return nil
		}
		scanned++
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(b), "QUORUM_") {
			rel, _ := filepath.Rel(root, path)
			t.Errorf("%s names a QUORUM_ variable; read it through env.Get and name FLMNT_ in text", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if scanned < 50 {
		t.Fatalf("swept only %d files; the walk is not reaching the module", scanned)
	}
}
