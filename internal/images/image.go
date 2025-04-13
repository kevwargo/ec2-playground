package images

import "github.com/aws/aws-sdk-go-v2/service/ec2/types"

type Image struct {
	Spec    string
	ID      string
	Details *types.Image
}
