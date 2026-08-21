package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/oriser/regroup"

	"kevwargo/ec2-playground/internal/infra"
)

func (r InstanceRunner) resolveNetworking(ctx context.Context, in *ec2.RunInstancesInput, resources infra.Resources) error {
	securityGroups := []string{resources.SecurityGroupEgress}

	rules, err := r.resolveIngressRules()
	if err != nil {
		return err
	}

	if len(rules) > 0 {
		ingressSecGroup, err := r.resolveResourceGroup(ctx, rules, resources)
		if err != nil {
			return err
		}

		securityGroups = append(securityGroups, ingressSecGroup)
	}

	if resources.Subnet != "" {
		in.NetworkInterfaces = []types.InstanceNetworkInterfaceSpecification{
			{
				DeviceIndex:              aws.Int32(0),
				Groups:                   securityGroups,
				AssociatePublicIpAddress: aws.Bool(!r.cfg.SkipPublicIPv4),
				SubnetId:                 &resources.Subnet,
			},
		}
	} else {
		in.SecurityGroupIds = securityGroups
	}

	return nil
}

func (r InstanceRunner) resolveIngressRules() ([]ingressRule, error) {
	if len(r.cfg.IPv4Ingress) == 0 {
		return nil, nil
	}

	var (
		rules     []ingressRule
		currentIP string
	)

	for _, arg := range r.cfg.IPv4Ingress {
		rule, err := parseIngressRule(arg)
		if err != nil {
			return nil, err
		}

		if rule.IP == "current" {
			if currentIP == "" {
				currentIP, err = getCurrentIP()
				if err != nil {
					return nil, err
				}
			}

			rule.IP = currentIP
		}

		r.sess.Log("Using rule: %+v", rule)

		rules = append(rules, rule)
	}

	return rules, nil
}

func (r InstanceRunner) resolveResourceGroup(ctx context.Context, rules []ingressRule, resources infra.Resources) (string, error) {
	name := fmt.Sprintf("EC2Playground-ingress-%s", computeHash(rules))
	r.sess.Log("secgroup for ingress rules(%v) - %s", rules, name)

	secGroupId, err := r.checkSecGroup(ctx, name)
	if err != nil {
		return "", err
	}

	if secGroupId == "" {
		r.sess.Log("Creating security group %s ...", name)
		secGroupId, err = r.createSecGroup(ctx, name, rules, resources)
	} else {
		r.sess.Log("Using security group %s (%s) ...", name, secGroupId)
	}

	return secGroupId, err
}

func (r InstanceRunner) checkSecGroup(ctx context.Context, name string) (string, error) {
	resp, err := r.sess.EC2().DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{{
			Name:   new("group-name"),
			Values: []string{name},
		}},
	})
	if err != nil {
		return "", err
	}

	if len(resp.SecurityGroups) > 0 {
		return *resp.SecurityGroups[0].GroupId, nil
	}

	return "", nil
}

func (r InstanceRunner) createSecGroup(
	ctx context.Context,
	name string,
	rules []ingressRule,
	resources infra.Resources,
) (string, error) {
	createResp, err := r.sess.EC2().CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   &name,
		Description: new("Custom ingress group"),
		VpcId:       &resources.VpcId,
	})
	if err != nil {
		return "", err
	}

	secGroupId := *createResp.GroupId

	authReq := ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: &secGroupId,
	}

	for _, rule := range rules {
		authReq.IpPermissions = append(authReq.IpPermissions, types.IpPermission{
			FromPort:   new(int32(rule.FromPort)),
			ToPort:     new(int32(rule.ToPort)),
			IpProtocol: &rule.Protocol,
			IpRanges:   []types.IpRange{{CidrIp: new(fmt.Sprintf("%s/%d", rule.IP, rule.Mask))}},
		})
	}

	_, err = r.sess.EC2().AuthorizeSecurityGroupIngress(ctx, &authReq)
	if err != nil {
		return "", err
	}

	return secGroupId, nil
}

type ingressRule struct {
	IP       string
	Mask     int
	FromPort int
	ToPort   int
	Protocol string
}

func parseIngressRule(raw string) (resp ingressRule, err error) {
	groups, err := ipIngressRegex.Groups(raw)
	if err != nil {
		return resp, fmt.Errorf("invalid ipv4 ingress rule %q: %w", raw, err)
	}

	if proto := groups["proto"]; proto == "" {
		resp.Protocol = "tcp"
	} else {
		resp.Protocol = proto
	}

	addr := groups["addr"]
	if addr != "current" {
		if net.ParseIP(addr) == nil {
			return resp, fmt.Errorf("address %q is invalid", addr)
		}
	}

	resp.IP = addr

	if mask := groups["mask"]; mask != "" {
		m, _ := strconv.Atoi(mask)
		if m < 0 || m > 32 {
			return ingressRule{}, fmt.Errorf("mask %q is invalid", mask)
		}

		resp.Mask = m
	} else {
		resp.Mask = 32
	}

	fromport, _ := strconv.Atoi(groups["fromport"])
	toport := fromport
	if tp := groups["toport"]; tp != "" {
		toport, _ = strconv.Atoi(tp)
	}

	if fromport < -1 || fromport > 65535 {
		return ingressRule{}, fmt.Errorf("FromPort %d is invalid", fromport)
	}
	if toport < -1 || toport > 65535 {
		return ingressRule{}, fmt.Errorf("ToPort %d is invalid", toport)
	}

	if toport < fromport {
		resp.FromPort = toport
		resp.ToPort = fromport
	} else {
		resp.FromPort = fromport
		resp.ToPort = toport
	}

	return resp, nil
}

func getCurrentIP() (string, error) {
	resp, err := http.Get("https://ifconfig.co/json")
	if err != nil {
		return "", fmt.Errorf("failed to fetch current IP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if result.IP == "" {
		return "", fmt.Errorf("empty IP in response")
	}

	return result.IP, nil
}

var ipIngressRegex = regroup.MustCompile("^((?P<proto>tcp|udp)://)?(?P<addr>[^/]+)(/(?P<mask>[0-9]{1,2}))?:(?P<fromport>[0-9]+)(-(?P<toport>[0-9]+))?$")
