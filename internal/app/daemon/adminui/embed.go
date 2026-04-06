package adminui

import (
	"embed"
	"io/fs"
)

//go:embed dist
var embeddedAssets embed.FS

func FS() fs.FS {
	dist, err := fs.Sub(embeddedAssets, "dist")
	if err != nil {
		panic(err)
	}
	return dist
}

func IndexHTML() ([]byte, error) {
	return fs.ReadFile(FS(), "index.html")
}
