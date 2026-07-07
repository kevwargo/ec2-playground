package rdp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"unicode"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/spf13/cobra"

	"kevwargo/ec2-playground/internal/session"
)

func Command(sess *session.Global) *cobra.Command {
	var cfg config

	cmd := &cobra.Command{
		Use:  "rdp INSTANCE_NAME_OR_ID",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.instanceSpec = args[0]

			return sess.RunSingle(cmd.Context(), func(ctx context.Context, r *session.Regional) error {
				return execute(ctx, r.EC2(), cfg)
			})
		},
	}

	cmd.Flags().StringVarP(&cfg.sshKeyFile, flagSSHKeyFile, "s", "", "SSH key to decrypt the password")
	cmd.MarkFlagRequired(flagSSHKeyFile)

	return cmd
}

type config struct {
	instanceSpec string
	sshKeyFile   string
}

type rdpData struct {
	publicIP string
	password string
}

func execute(ctx context.Context, ec2Client *ec2.Client, cfg config) error {
	rdpData, err := getRDPData(ctx, ec2Client, cfg)
	if err != nil {
		return err
	}

	return runRemmina(rdpData)
}

func getRDPData(ctx context.Context, ec2Client *ec2.Client, cfg config) (rdpData, error) {
	instance, err := getMatchingInstance(ctx, ec2Client, cfg.instanceSpec)
	if err != nil {
		return rdpData{}, err
	}

	instanceID := *instance.InstanceId
	log.Printf("Found instance %s", instanceID)

	publicIP := instance.PublicIpAddress
	if publicIP == nil {
		return rdpData{}, fmt.Errorf("instance %s does not have a public IP", instanceID)
	}

	pwdResp, err := ec2Client.GetPasswordData(ctx, &ec2.GetPasswordDataInput{InstanceId: &instanceID})
	if err != nil {
		return rdpData{}, err
	}

	if pwdResp.PasswordData == nil || *pwdResp.PasswordData == "" {
		return rdpData{}, fmt.Errorf("password data for %s is not ready yet", instanceID)
	}

	password, err := decryptEC2Password(*pwdResp.PasswordData, cfg.sshKeyFile)
	if err != nil {
		return rdpData{}, err
	}

	return rdpData{
		publicIP: *publicIP,
		password: password,
	}, nil
}

func getMatchingInstance(ctx context.Context, ec2Client *ec2.Client, instanceSpec string) (types.Instance, error) {
	params := ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("platform"),
				Values: []string{"windows"},
			},
			{
				Name:   aws.String("instance-state-name"),
				Values: []string{"running"},
			},
		},
	}

	if strings.HasPrefix(instanceSpec, "i-") {
		params.InstanceIds = []string{instanceSpec}
	} else {
		params.Filters = append(params.Filters, types.Filter{
			Name:   aws.String("tag:Name"),
			Values: []string{instanceSpec},
		})
	}

	resp, err := ec2Client.DescribeInstances(ctx, &params)
	if err != nil {
		return types.Instance{}, err
	}

	var instances []types.Instance
	for _, r := range resp.Reservations {
		instances = append(instances, r.Instances...)
	}
	if len(instances) == 0 {
		return types.Instance{}, fmt.Errorf("instance %q is not a running Windows instance", instanceSpec)
	}
	if len(instances) > 1 {
		return types.Instance{}, fmt.Errorf("there are more than one running instance with the name %q", instanceSpec)
	}

	return instances[0], nil
}

func decryptEC2Password(encPassword, sshKeyFile string) (string, error) {
	var (
		inBuf  = base64.NewDecoder(base64.StdEncoding, strings.NewReader(encPassword))
		outBuf bytes.Buffer
	)

	cmd := exec.Command("openssl", "pkeyutl", "-decrypt", "-inkey", sshKeyFile)
	cmd.Stdin = inBuf
	cmd.Stdout = &outBuf
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", err
	}

	password := outBuf.String()
	for _, c := range password {
		if !unicode.IsPrint(c) {
			return "", fmt.Errorf("ssh keyfile %s doesn't seem to be the correct one", sshKeyFile)
		}
	}

	return password, nil
}

func runRemmina(data rdpData) error {
	uri, err := buildRemminaURI(data)
	if err != nil {
		return err
	}

	return exec.Command(remminaCmd, "-c", uri).Start()
}

func buildRemminaURI(data rdpData) (string, error) {
	var output bytes.Buffer

	cmd := initRemminaEncryptionCmd(data.password, &output)

	if err := cmd.Run(); err != nil {
		if ee, ok := errors.AsType[*exec.ExitError](err); ok {
			if ee.ExitCode() != 0 && ee.ExitCode() != 1 {
				return "", err
			}
		}
	}

	foundUsage := false
	scanner := bufio.NewScanner(&output)
	for scanner.Scan() {
		if foundUsage {
			if m := rdpURIRegexp.FindStringSubmatch(scanner.Text()); m != nil {
				return fmt.Sprintf("rdp://%s:%s@%s", username, m[1], data.publicIP), nil
			}
		} else if scanner.Text() == "Usage:" {
			foundUsage = true
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", fmt.Errorf("%s not found in the remmina output", rdpURIRegexp.String())
}

func initRemminaEncryptionCmd(password string, outbuf *bytes.Buffer) *exec.Cmd {
	cmd := exec.Command(remminaCmd, "--encrypt-password")
	cmd.Stdin = strings.NewReader(password)
	cmd.Stdout = outbuf
	cmd.Stderr = os.Stderr

	var (
		env       []string
		foundDBUS bool
	)

	for _, entry := range cmd.Environ() {
		if strings.HasPrefix(entry, dbusEnvPrefix) {
			env = append(env, dbusEnvPrefix)
			foundDBUS = true
		} else {
			env = append(env, entry)
		}
	}

	if !foundDBUS {
		env = append(env, dbusEnvPrefix)
	}

	cmd.Env = env

	return cmd
}

const (
	flagSSHKeyFile = "ssh-key-file"
	remminaCmd     = "remmina"
	username       = "Administrator"
	dbusEnvPrefix  = "DBUS_SESSION_BUS_ADDRESS="
)

var rdpURIRegexp = regexp.MustCompile("^rdp://username:(.+)@server$")
