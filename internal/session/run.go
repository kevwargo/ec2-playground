package session

import (
	"context"
	"errors"
	"fmt"
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
		errs[i] = <-errsC
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

	return run(ctx, g.defaultRegional)
}
