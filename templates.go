package main

import (
	"embed"
	"html/template"

	"github.com/Top-Ranger/questiongo/translation"
)

//go:embed template
var templateFiles embed.FS

var textTemplate *template.Template
var errorTemplate *template.Template

var evenOddFuncMap = template.FuncMap{
	"even": func(i int) bool {
		return i%2 == 0
	},
}

type errorTemplateStruct struct {
	Error       template.HTML
	Translation translation.Translation
	ServerPath  string
}

type textTemplateStruct struct {
	Text        template.HTML
	Translation translation.Translation
	ServerPath  string
}

func init() {
	var err error

	errorTemplate, err = template.ParseFS(templateFiles, "template/error.html")
	if err != nil {
		panic(err)
	}

	textTemplate, err = template.ParseFS(templateFiles, "template/text.html")
	if err != nil {
		panic(err)
	}
}
