package userdata

import (
	_ "embed"
	"encoding/base64"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func Build(spec string) (*string, error) {
	if spec == "" {
		return nil, nil
	}

	body, exists := builtinModules[spec]
	if !exists {
		var err error
		body, err = os.ReadFile(spec)
		if err != nil {
			return nil, err
		}
	}

	return aws.String(base64.StdEncoding.EncodeToString(body)), nil
}

//go:embed enable-ssm.sh
var templateSSMAgentLinux []byte

var builtinModules = map[string][]byte{
	"enable-ssm": templateSSMAgentLinux,
}
