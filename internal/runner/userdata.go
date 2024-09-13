package runner

import (
	"encoding/base64"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

func (r InstanceRunner) setUserData(in *ec2.RunInstancesInput) error {
	if r.cfg.UserData == "" {
		return nil
	}

	body, err := os.ReadFile(r.cfg.UserData)
	if err != nil {
		return err
	}

	in.UserData = aws.String(base64.StdEncoding.EncodeToString(body))
	return nil
}
