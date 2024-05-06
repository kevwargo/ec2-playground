package runner

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
)

type profileBuilder struct {
	iam           *iam.Client
	userPolicy    string
	defaultPolicy string
	infraName     string
}

func (b *profileBuilder) buildProfile(ctx context.Context) (string, error) {
	policyHash, err := b.getPolicyHash(ctx, b.userPolicy)
	if err != nil {
		return "", err
	}

	log.Printf("hash(%s) = %s", b.userPolicy, policyHash)

	profileName := fmt.Sprintf(instanceProfileFmt, b.infraName, policyHash)
	if exists, err := b.profileExists(ctx, profileName); exists {
		return profileName, nil
	} else if err != nil {
		return "", err
	}

	role, err := b.createRole(ctx, policyHash)
	if err != nil {
		return "", err
	}

	if err := b.createProfile(ctx, profileName, role); err != nil {
		return "", err
	}

	return profileName, nil
}

func (b *profileBuilder) profileExists(ctx context.Context, name string) (bool, error) {
	_, err := b.iam.GetInstanceProfile(ctx, &iam.GetInstanceProfileInput{InstanceProfileName: &name})
	if err == nil {
		return true, nil
	}

	var nse *types.NoSuchEntityException
	if errors.As(err, &nse) {
		return false, nil
	}

	return false, err
}

type policyDocument struct {
	Version   string
	Statement []policyStatement
}

type policyStatement struct {
	Action    string
	Effect    string
	Principal principal
}

type principal struct {
	Service string
}

func (b *profileBuilder) createRole(ctx context.Context, policyHash string) (string, error) {
	roleName := fmt.Sprintf(instanceRoleFmt, b.infraName, policyHash)

	assumeDocument, err := json.Marshal(policyDocument{
		Version: "2012-10-17",
		Statement: []policyStatement{
			{
				Action:    "sts:AssumeRole",
				Effect:    "Allow",
				Principal: principal{Service: "ec2.amazonaws.com"},
			},
		},
	})
	if err != nil {
		return "", err
	}

	_, err = b.iam.CreateRole(ctx, &iam.CreateRoleInput{
		RoleName:                 &roleName,
		AssumeRolePolicyDocument: aws.String(string(assumeDocument)),
	})
	if err != nil {
		return "", err
	}
	log.Printf("Role %s created", roleName)

	if err := b.attachPolicy(ctx, b.userPolicy, roleName); err != nil {
		return "", err
	}
	if err := b.attachPolicy(ctx, b.defaultPolicy, roleName); err != nil {
		return "", err
	}
	if err := b.attachPolicy(ctx, ssmPolicy, roleName); err != nil {
		return "", err
	}
	log.Printf("Attached policies to %s", roleName)

	return roleName, nil
}

func (b *profileBuilder) attachPolicy(ctx context.Context, policyArn, roleName string) error {
	_, err := b.iam.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
		PolicyArn: &policyArn,
		RoleName:  &roleName,
	})
	return err
}

func (b *profileBuilder) createProfile(ctx context.Context, profileName, roleName string) error {
	_, err := b.iam.CreateInstanceProfile(ctx, &iam.CreateInstanceProfileInput{
		InstanceProfileName: &profileName,
	})
	if err != nil {
		return err
	}
	log.Printf("Profile %s created", profileName)

	_, err = b.iam.AddRoleToInstanceProfile(ctx, &iam.AddRoleToInstanceProfileInput{
		InstanceProfileName: &profileName,
		RoleName:            &roleName,
	})
	if err != nil {
		return err
	}
	log.Printf("Role %s added to profile %s", roleName, profileName)

	return nil
}

func (b *profileBuilder) getPolicyHash(ctx context.Context, policy string) (string, error) {
	policyResp, err := b.iam.GetPolicy(ctx, &iam.GetPolicyInput{PolicyArn: &policy})
	if err != nil {
		return "", err
	}

	versionResp, err := b.iam.GetPolicyVersion(ctx, &iam.GetPolicyVersionInput{
		PolicyArn: &policy,
		VersionId: policyResp.Policy.DefaultVersionId,
	})
	if err != nil {
		return "", err
	}

	h := md5.New()
	h.Write([]byte(*versionResp.PolicyVersion.Document))
	policyHash := base64.RawURLEncoding.EncodeToString(h.Sum(nil))
	return policyHash, nil
}

const (
	ssmPolicy = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"

	instanceProfileFmt = "%s-InstanceProfile-%s"
	instanceRoleFmt    = "%s-InstanceRole-%s"
)
