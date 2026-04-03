package ssmcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Document     Document
	Params       Parameters
	InlineScript string
	ScriptFile   string
	OutputsDir   string
}

func (c Config) BuildParams() (Parameters, error) {
	params := c.Params

	switch c.Document.name {
	case docPowerShellScript, docShellScript:
		if c.InlineScript != "" {
			return Parameters{"commands": []string{c.InlineScript}}, nil
		}
		if c.ScriptFile != "" {
			body, err := os.ReadFile(c.ScriptFile)
			if err != nil {
				return nil, err
			}

			body = bytes.ReplaceAll(body, []byte{'\r'}, []byte{})

			if params == nil {
				params = make(Parameters)
			}
			for _, line := range bytes.Split(body, []byte{'\n'}) {
				params["commands"] = append(params["commands"], string(line))
			}
		}
	default:
		if c.InlineScript != "" || c.ScriptFile != "" {
			return nil, fmt.Errorf("script can only be specified for %s and %s documents", docPowerShellScript, docShellScript)
		}
	}

	return params, nil
}

type Parameters map[string][]string

func (p Parameters) Type() string {
	return "SSM document parameters"
}

func (p Parameters) String() string {
	b, _ := json.Marshal(p)
	return string(b)
}

func (p *Parameters) Set(raw string) error {
	if *p == nil {
		*p = make(Parameters)
	}

	parts := strings.SplitN(raw, "=", 2)
	if len(parts) < 2 {
		return fmt.Errorf("invalid document parameter specifier: %q", raw)
	}

	(*p)[parts[0]] = append((*p)[parts[0]], parts[1])

	return nil
}

type Document struct {
	name string
}

func (d *Document) Set(raw string) error {
	switch raw {
	case "sh", "shell":
		d.name = docShellScript
	case "ps", "powershell":
		d.name = docPowerShellScript
	default:
		d.name = raw
	}

	return nil
}

func (d Document) String() string {
	return d.name
}

func (d Document) Type() string {
	return "ssm-document"
}

func (d Document) Name() *string {
	return &d.name
}

const (
	docShellScript      = "AWS-RunShellScript"
	docPowerShellScript = "AWS-RunPowerShellScript"
)
