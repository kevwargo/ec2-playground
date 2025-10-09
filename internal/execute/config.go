package execute

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Config struct {
	Document   string
	Params     Parameters
	OutputsDir string
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
