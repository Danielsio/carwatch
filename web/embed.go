package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFiles embed.FS

func DistFS() fs.FS {
	sub, err := fs.Sub(distFiles, "dist")
	if err != nil {
		panic("web: embed dist: " + err.Error())
	}
	return sub
}
