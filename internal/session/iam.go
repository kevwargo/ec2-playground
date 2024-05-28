package session

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/iam"
)

func (g *Global) RunIAM(ctx context.Context, run func(context.Context, *iam.Client) error) error {
	g.iamMutex.Lock()
	defer g.iamMutex.Unlock()

	if err := g.init(ctx); err != nil {
		return err
	}

	if g.iam == nil {
		g.iam = iam.NewFromConfig(g.defaultRegional.cfg)
	}

	return run(ctx, g.iam)
}
