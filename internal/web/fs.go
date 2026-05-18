package web

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var (
	distOnce     sync.Once
	distFS       fs.FS
	distSource   string
	distFromDisk bool
	distErr      error
)

// DistFS returns the UI asset filesystem and a short source label for logs.
// Disk is preferred when COPILOT_PROXY_UI_DIST is set or internal/web/dist exists
// (so frontend npm run build applies without recompiling Go).
func DistFS() (fs.FS, string, error) {
	distOnce.Do(func() {
		distFS, distSource, distFromDisk, distErr = resolveDistFS()
		if distErr == nil {
			log.Printf("[web] serving UI from %s (disk=%v)", distSource, distFromDisk)
			log.Printf("[web] embedded file list:")
			fs.WalkDir(distFS, ".", func(path string, d fs.DirEntry, err error) error {
				if err == nil && !d.IsDir() {
					log.Printf("  → %s", path)
				}
				return nil
			})
		}
	})
	return distFS, distSource, distErr
}

// DistFromDisk reports whether the UI is served from the filesystem instead of go:embed.
func DistFromDisk() bool {
	_, _, _ = DistFS()
	return distFromDisk
}

func resolveDistFS() (fs.FS, string, bool, error) {
	if dir := os.Getenv("COPILOT_PROXY_UI_DIST"); dir != "" {
		if ok, err := uiDistReady(dir); err != nil {
			return nil, "", false, err
		} else if ok {
			return os.DirFS(dir), dir, true, nil
		}
		return nil, "", false, fmt.Errorf("COPILOT_PROXY_UI_DIST=%q: missing index.html", dir)
	}

	for _, dir := range uiDistCandidates() {
		if ok, _ := uiDistReady(dir); ok {
			abs, err := filepath.Abs(dir)
			if err != nil {
				abs = dir
			}
			return os.DirFS(abs), abs, true, nil
		}
	}

	sub, err := fs.Sub(embeddedAssets, "dist")
	if err != nil {
		return nil, "", false, err
	}
	return sub, "go:embed (rebuild Go binary after frontend changes)", false, nil
}

func uiDistCandidates() []string {
	seen := make(map[string]struct{})
	add := func(list *[]string, dir string) {
		if dir == "" {
			return
		}
		if _, ok := seen[dir]; ok {
			return
		}
		seen[dir] = struct{}{}
		*list = append(*list, dir)
	}

	var out []string
	if cwd, err := os.Getwd(); err == nil {
		add(&out, filepath.Join(cwd, "internal/web/dist"))
	}
	if exec, err := os.Executable(); err == nil {
		execDir := filepath.Dir(exec)
		add(&out, filepath.Join(execDir, "internal/web/dist"))
		add(&out, filepath.Join(execDir, "dist"))
	}
	return out
}

func uiDistReady(dir string) (bool, error) {
	info, err := os.Stat(filepath.Join(dir, "index.html"))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return !info.IsDir(), nil
}
