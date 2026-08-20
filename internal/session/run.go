package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/smithy-go"
)

func (g *Global) Run(ctx context.Context, run func(context.Context, *Regional) error) error {
	if err := g.init(ctx); err != nil {
		return err
	}

	errsC := make(chan error)
	for _, session := range g.regional {
		go func(session *Regional) {
			errsC <- run(ctx, session)
		}(session)
	}

	errs := make([]error, len(g.regional))
	for i := range errs {
		errs[i] = g.maybeIgnoreError(<-errsC)
	}

	return errors.Join(errs...)
}

func (g *Global) RunSingle(ctx context.Context, run func(context.Context, *Regional) error) error {
	if err := g.init(ctx); err != nil {
		return err
	}

	if len(g.regional) != 1 {
		return fmt.Errorf("only single region can be specified in this operation")
	}

	return g.maybeIgnoreError(run(ctx, g.defaultRegional))
}

func (g *Global) maybeIgnoreError(err error) error {
	if !g.IgnoreAccessErrors || err == nil {
		return err
	}

	var (
		opErr  *smithy.OperationError
		apiErr smithy.APIError
	)

	if !(errors.As(err, &opErr) && errors.As(err, &apiErr)) {
		return err
	}

	for _, ignored := range ignoredAccessErrors {
		if opErr.ServiceID == ignored.service && apiErr.ErrorCode() == ignored.errorCode {
			return nil
		}
	}

	return err
}

type awsErrorPattern struct {
	service   string
	errorCode string
}

var ignoredAccessErrors = []awsErrorPattern{
	{"EC2", "UnauthorizedOperation"},
	{"EC2", "AuthFailure"},
	{"CloudFormation", "AccessDenied"},
	{"SSM", "AccessDeniedException"},
}
