package session

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func (s *Session) Run(ctx context.Context, run func(context.Context, aws.Config) error) error {
	errsC := make(chan error)
	for _, cfg := range s.configs {
		go func(cfg aws.Config) {
			errsC <- run(ctx, cfg)
		}(cfg)
	}

	errs := make([]error, 0, len(s.configs))
	for len(errs) < len(s.configs) {
		errs = append(errs, <-errsC)
	}

	return errors.Join(errs...)
}
