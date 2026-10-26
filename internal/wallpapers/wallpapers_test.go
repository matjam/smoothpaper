package wallpapers

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// ScanWallpapers must expand "~", match the extension in any case, skip a
// directory even when its name looks like an image, and not scan subdirectories.
func TestScanWallpapers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	for _, name := range []string{
		"walls/a.PNG",
		"walls/b.jpg",
		"walls/c.JPEG",
		"walls/d.gif",
		"walls/e.webp",
		"walls/notes.txt",
		"walls/folder.png/nested.png",
		"more/f.png",
	} {
		path := filepath.Join(home, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := ScanWallpapers([]string{"~/walls", filepath.Join(home, "more")})
	if err != nil {
		t.Fatalf("ScanWallpapers() error: %v", err)
	}

	want := []string{
		filepath.Join(home, "walls/a.PNG"),
		filepath.Join(home, "walls/b.jpg"),
		filepath.Join(home, "walls/c.JPEG"),
		filepath.Join(home, "walls/d.gif"),
		filepath.Join(home, "more/f.png"),
	}
	if !slices.Equal(got, want) {
		t.Errorf("ScanWallpapers() =\n%q\nwant\n%q", got, want)
	}
}
