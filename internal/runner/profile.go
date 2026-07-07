package runner

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"

	"kevwargo/ec2-playground/internal/infra"
	"kevwargo/ec2-playground/internal/session"
)

func (r InstanceRunner) setProfile(ctx context.Context, in *ec2.RunInstancesInput, resources infra.Resources) (wait bool, err error) {
	var profile string

	if r.cfg.Profile != "" {
		profile = r.cfg.Profile
	} else if len(r.cfg.Policies) > 0 {
		err = r.sess.Global.RunIAM(ctx, func(ctx context.Context, iamClient *iam.Client) error {
			builder := profileBuilder{
				session:       r.sess.Global,
				iam:           iamClient,
				region:        r.sess.Region,
				userPolicies:  r.cfg.Policies,
				defaultPolicy: resources.InstancePolicy,
				infraName:     r.cfg.Infra.StackName,
			}

			profileName, err := builder.buildProfile(ctx)
			if err == nil {
				profile = profileName
			}

			return err
		})
		if err != nil {
			return false, err
		}
		wait = true
	} else {
		profile = resources.InstanceProfile
	}

	in.IamInstanceProfile = &ec2types.IamInstanceProfileSpecification{
		Name: &profile,
	}

	return wait, nil
}

type profileBuilder struct {
	session       *session.Global
	iam           *iam.Client
	region        string
	userPolicies  []string
	defaultPolicy string
	infraName     string
}

func (b *profileBuilder) buildProfile(ctx context.Context) (string, error) {
	policiesHash := b.computePoliciesHash()

	profileName := fmt.Sprintf(instanceProfileFmt, b.region, policiesHash)
	if exists, err := b.profileExists(ctx, profileName); exists {
		return profileName, nil
	} else if err != nil {
		return "", err
	}

	role, err := b.createRole(ctx, policiesHash)
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
		b.session.Log("Instance profile %s already exists", name)

		return true, nil
	}

	if _, ok := errors.AsType[*types.NoSuchEntityException](err); ok {
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
	roleName := fmt.Sprintf(instanceRoleFmt, b.region, policyHash)

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
	b.session.Log("Role %s created", roleName)

	for _, userPolicy := range b.userPolicies {
		if err := b.attachPolicy(ctx, userPolicy, roleName); err != nil {
			return "", err
		}
	}

	if err := b.attachPolicy(ctx, b.defaultPolicy, roleName); err != nil {
		return "", err
	}
	if err := b.attachPolicy(ctx, ssmPolicy, roleName); err != nil {
		return "", err
	}

	b.session.Log("Attached policies to %s", roleName)

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
	b.session.Log("Profile %s created", profileName)

	_, err = b.iam.AddRoleToInstanceProfile(ctx, &iam.AddRoleToInstanceProfileInput{
		InstanceProfileName: &profileName,
		RoleName:            &roleName,
	})
	if err != nil {
		return err
	}
	b.session.Log("Role %s added to profile %s", roleName, profileName)

	return nil
}

func (b *profileBuilder) computePoliciesHash() string {
	h := md5.New()

	policies := []string{b.defaultPolicy, ssmPolicy}
	policies = append(policies, b.userPolicies...)
	slices.Sort(policies)

	for _, policy := range policies {
		h.Write([]byte(policy))
	}

	policiesHash := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	b.session.Log("hash(%s) = %s", policies, policiesHash)

	return policiesHash
}

const (
	ssmPolicy = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"

	instanceProfileFmt = "ec2pg-InstanceProfile-%s-%s"
	instanceRoleFmt    = "ec2pg-InstanceRole-%s-%s"
)
