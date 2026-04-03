package vmformat

import (
	"bytes"
	"context"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"kevwargo/ec2-playground/internal/session"
)

type VM struct {
	types.Instance

	format string
}

func (v VM) String() string {
	return v.format
}

type Formatter struct {
	session  *session.Regional
	tmpl     *template.Template
	ssmCache *ssmCache
	amiCache map[string]types.Image
}

func New(sess *session.Regional, tmpl *template.Template) Formatter {
	return Formatter{
		session:  sess,
		tmpl:     tmpl,
		ssmCache: initSSMCache(sess),
		amiCache: make(map[string]types.Image),
	}
}

func (f Formatter) Format(ctx context.Context, instance types.Instance) (VM, error) {
	var buf bytes.Buffer
	err := f.tmpl.Execute(&buf, f.prepareInstanceData(ctx, instance))
	if err != nil {
		return VM{}, err
	}

	return VM{
		Instance: instance,
		format:   buf.String(),
	}, nil
}
