package config

import (
	"log"
	"text/template"
)

type VMFormat struct {
	tmpl *template.Template
}

func (t *VMFormat) String() string {
	return defaultFormat
}

func (t *VMFormat) Type() string {
	return "template"
}

func (t *VMFormat) Set(raw string) error {
	tmpl, err := template.New("param").Parse(raw)
	if err != nil {
		return err
	}

	t.tmpl = tmpl

	return nil
}

func (t *VMFormat) Template() *template.Template {
	if t.tmpl != nil {
		return t.tmpl
	}

	return defaultTemplate
}

const defaultFormat = "{{.Region}} {{.Id}} {{.Name}} {{.State}}"

var defaultTemplate *template.Template

func init() {
	t, err := template.New("default").Parse(defaultFormat)
	if err != nil {
		log.Fatalf("error in default format template: %s", err.Error())
	}

	defaultTemplate = t
}
