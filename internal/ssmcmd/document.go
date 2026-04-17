package ssmcmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Document struct {
	Name         string
	Params       Parameters
	InlineScript string
	ScriptFile   string
}

// resolve normalizes d.Name and resolves d.InlineScript or d.ScriptFile in-place if applicable.
func (d *Document) resolve() error {
	switch d.Name {
	case "sh", "shell", docShellScript:
		d.Name = docShellScript
	case "ps", "powershell", docPowerShellScript:
		d.Name = docPowerShellScript
	default:
		if d.InlineScript != "" || d.ScriptFile != "" {
			return fmt.Errorf("script can only be specified for %s or %s documents", docPowerShellScript, docShellScript)
		}
	}

	switch d.Name {
	case docPowerShellScript, docShellScript:
		if err := d.resolveScriptContent(); err != nil {
			return err
		}
	}

	return nil
}

func (d *Document) resolveScriptContent() error {
	if d.Params == nil {
		d.Params = make(Parameters)
	}

	if d.InlineScript != "" {
		d.Params["commands"] = []string{d.InlineScript}
	} else if d.ScriptFile != "" {
		body, err := os.ReadFile(d.ScriptFile)
		if err != nil {
			return err
		}

		body = bytes.ReplaceAll(body, []byte{'\r'}, []byte{})

		d.Params["commands"] = make([]string, 0)
		for _, line := range bytes.Split(body, []byte{'\n'}) {
			d.Params["commands"] = append(d.Params["commands"], string(line))
		}
	} else if len(d.Params["commands"]) == 0 {
		return errors.New("refusing to run empty script")
	}

	return nil
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

const (
	docShellScript      = "AWS-RunShellScript"
	docPowerShellScript = "AWS-RunPowerShellScript"
)
