package web

import "embed"

//go:embed all:dist
var embeddedAssets embed.FS
