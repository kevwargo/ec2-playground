package session

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type Regional struct {
	Region string
	Global *Global

	cfg aws.Config
	ec2 *ec2.Client
	ssm *ssm.Client
	cfn *cloudformation.Client
	s3  *s3.Client
	sqs *sqs.Client
}

func (g *Global) newRegional(cfg aws.Config) *Regional {
	return &Regional{
		Region: cfg.Region,
		Global: g,

		cfg: cfg,
	}
}

func (s *Regional) EC2() *ec2.Client {
	if s.ec2 == nil {
		s.ec2 = ec2.NewFromConfig(s.cfg)
	}

	return s.ec2
}

func (s *Regional) SSM() *ssm.Client {
	if s.ssm == nil {
		s.ssm = ssm.NewFromConfig(s.cfg)
	}

	return s.ssm
}

func (s *Regional) CFN() *cloudformation.Client {
	if s.cfn == nil {
		s.cfn = cloudformation.NewFromConfig(s.cfg)
	}

	return s.cfn
}

func (s *Regional) S3() *s3.Client {
	if s.s3 == nil {
		s.s3 = s3.NewFromConfig(s.cfg)
	}

	return s.s3
}

func (s *Regional) SQS() *sqs.Client {
	if s.sqs == nil {
		s.sqs = sqs.NewFromConfig(s.cfg)
	}

	return s.sqs
}
