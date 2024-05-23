package session

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/iam"
)

func (s *Session) RunIAM(ctx context.Context, run func(context.Context, *iam.Client) error) error {
	s.iamMutex.Lock()
	defer s.iamMutex.Unlock()

	if s.iam == nil {
		s.iam = iam.NewFromConfig(s.configs[0])
	}

	return run(ctx, s.iam)
}
