package config

import (
	"log"
	"text/template"
)

type InstanceFormat struct {
	tmpl *template.Template
}

func (t *InstanceFormat) String() string {
	return defaultFormat
}

func (t *InstanceFormat) Type() string {
	return "template"
}

func (t *InstanceFormat) Set(raw string) error {
	tmpl, err := template.New("param").Parse(raw)
	if err != nil {
		return err
	}

	t.tmpl = tmpl

	return nil
}

func (t *InstanceFormat) Template() *template.Template {
	if t.tmpl != nil {
		return t.tmpl
	}

	return defaultTemplate
}

const defaultFormat = "{{.Region}} {{.Id}} {{.Name}} {{.I.State.Name}}"

var defaultTemplate *template.Template

func init() {
	t, err := template.New("default").Parse(defaultFormat)
	if err != nil {
		log.Fatalf("error in default format template: %s", err.Error())
	}

	defaultTemplate = t
}
