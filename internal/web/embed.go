package web

import (
	"embed"
	"html/template"
	"io/fs"
)

//go:embed templates/*.html
var templateEmbed embed.FS

//go:embed static/*
var staticEmbed embed.FS

func LoadTemplates(funcMap template.FuncMap) (*template.Template, error) {
	return template.New("archive").Funcs(funcMap).ParseFS(templateEmbed, "templates/*.html")
}

func StaticFS() fs.FS {
	files, err := fs.Sub(staticEmbed, "static")
	if err != nil {
		panic(err)
	}
	return files
}
