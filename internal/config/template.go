package config

import (
	"fmt"
	"regexp"
	"strings"
	"text/template"
)

type VMFormat struct {
	value *string
	tmpl  *template.Template
}

func (t *VMFormat) String() string {
	if t.value != nil {
		return *t.value
	}
	if t.tmpl != nil {
		return t.tmpl.Root.String()
	}

	return strings.Join(defaultFields, ",")
}

func (t *VMFormat) Type() string {
	return "template"
}

func (t *VMFormat) Set(raw string) error {
	var (
		value string
		tmpl  *template.Template
		err   error
	)

	if m := regexFields.FindStringSubmatch(raw); m != nil {
		fields := strings.Split(m[2], ",")
		if m[1] == "+" {
			fields = append(defaultFields, fields...)
		}

		value = strings.Join(fields, ",")
		tmpl, err = buildFieldsTemplate(fields)
	} else {
		value = raw
		tmpl, err = template.New("param").Parse(raw)
	}

	if err != nil {
		return err
	}

	t.value = &value
	t.tmpl = tmpl

	return nil
}

func (t *VMFormat) Template() *template.Template {
	if t.tmpl != nil {
		return t.tmpl
	}

	return defaultTemplate
}

func buildFieldsTemplate(fields []string) (*template.Template, error) {
	templateFields := make([]string, 0, len(fields))

	for _, field := range fields {
		if resolved, exists := fieldAliases[field]; !exists {
			return nil, fmt.Errorf("format field %q is invalid (%v)", field, fields)
		} else {
			templateFields = append(templateFields, fmt.Sprintf("{{.%s}}", resolved))
		}
	}

	return template.New("fields").Parse(strings.Join(templateFields, " "))
}

var (
	defaultFields = []string{
		fieldRegion,
		fieldId,
		fieldName,
		fieldState,
	}
	defaultTemplate = template.Must(buildFieldsTemplate(defaultFields))

	regexFields  = regexp.MustCompile(`^(\+?)([a-z-]+(,[a-z-]+)*)$`)
	fieldAliases = map[string]string{
		fieldRegion: "Region",
		fieldId:     "Id",
		fieldName:   "Name",
		fieldState:  "State",
		fieldType:   "Type",
		fieldIP:     "I.PublicIpAddress",
		fieldPing:   "Ping",
		fieldLaunch: "I.LaunchTime",
		fieldTags:   "Tags",
	}
)

const (
	fieldRegion = "region"
	fieldId     = "id"
	fieldName   = "name"
	fieldState  = "state"
	fieldType   = "type"
	fieldIP     = "ip"
	fieldPing   = "ping"
	fieldLaunch = "launch"
	fieldTags   = "tags"
)
