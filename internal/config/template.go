package config

import (
	"log"
	"text/template"
)

type TemplateFlag struct {
	tmpl *template.Template
}

func (t *TemplateFlag) String() string {
	return "template"
}

func (t *TemplateFlag) Type() string {
	return "template"
}

func (t *TemplateFlag) Set(raw string) error {
	tmpl, err := template.New("param").Parse(raw)
	if err != nil {
		return err
	}

	t.tmpl = tmpl

	return nil
}

func (t *TemplateFlag) Template() *template.Template {
	if t.tmpl != nil {
		return t.tmpl
	}

	return defaultFormat
}

var defaultFormat *template.Template

func init() {
	t, err := template.New("default").Parse("{{.Region}} {{.Id}} {{.Name}}")
	if err != nil {
		log.Fatalf("error in default format template: %s", err.Error())
	}

	defaultFormat = t
}
