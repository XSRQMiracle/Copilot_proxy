package web

import (
	"embed"
	"io/fs"
	"sync"
)

//go:embed favicon.png
var faviconRaw embed.FS

var (
	faviconOnce   sync.Once
	faviconBytes  []byte
	faviconErr    error
)

func Favicon() ([]byte, error) {
	faviconOnce.Do(func() {
		faviconBytes, faviconErr = fs.ReadFile(faviconRaw, "favicon.png")
	})
	return faviconBytes, faviconErr
}
