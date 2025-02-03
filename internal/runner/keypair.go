package runner

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/smithy-go"
)

func (r InstanceRunner) setKeyPair(ctx context.Context, in *ec2.RunInstancesInput) error {
	if r.cfg.KeyPair != "" {
		in.KeyName = aws.String(r.cfg.KeyPair)
	} else if r.cfg.SSHPublicKeyFile != "" {
		return r.setPublicSSHKey(ctx, r.cfg.SSHPublicKeyFile, in)
	}

	return nil
}

type sshKey struct {
	data        []byte
	fingerprint []byte
	name        string
}

func (r InstanceRunner) setPublicSSHKey(ctx context.Context, keyFile string, in *ec2.RunInstancesInput) error {
	key, err := r.readSSHKey(keyFile)
	if err != nil {
		return err
	}

	if err := r.deployKey(ctx, key); err != nil {
		return err
	}

	in.KeyName = &key.name

	return nil
}

func (r InstanceRunner) deployKey(ctx context.Context, key sshKey) error {
	describeResp, err := r.sess.EC2().DescribeKeyPairs(ctx, &ec2.DescribeKeyPairsInput{
		KeyNames: []string{key.name},
	})
	if err != nil {
		if ae := smithy.APIError(nil); !errors.As(err, &ae) || ae.ErrorCode() != "InvalidKeyPair.NotFound" {
			return err
		}
	}

	if describeResp != nil && len(describeResp.KeyPairs) > 0 {
		k := describeResp.KeyPairs[0]
		r.sess.Log("Key %s(%s %s) exists", *k.KeyName, *k.KeyPairId, *k.KeyFingerprint)
		return nil
	}

	importResp, err := r.sess.EC2().ImportKeyPair(ctx, &ec2.ImportKeyPairInput{
		KeyName:           &key.name,
		PublicKeyMaterial: key.data,
	})
	if err != nil {
		return err
	}

	r.sess.Log("Imported new key: %s(%s %s)", *importResp.KeyName, *importResp.KeyPairId, *importResp.KeyFingerprint)

	return nil
}

func (r InstanceRunner) readSSHKey(keyFile string) (sshKey, error) {
	keyPEM, err := collectOutput(exec.Command("ssh-keygen", "-ef", keyFile, "-m", "PEM"))
	if err != nil {
		return sshKey{}, err
	}

	cmd := exec.Command("openssl", "rsa", "-RSAPublicKey_in", "-outform", "DER")
	cmd.Stdin = bytes.NewBuffer(keyPEM)
	keyDER, err := collectOutput(cmd)
	if err != nil {
		return sshKey{}, err
	}

	h := md5.New()
	h.Write(keyDER)
	fp := h.Sum(nil)

	var hex []string
	for _, b := range fp {
		hex = append(hex, fmt.Sprintf("%02x", b))
	}
	r.sess.Log("%s HEX fingerprint: %s", keyFile, strings.Join(hex, ":"))

	data, err := os.ReadFile(keyFile)
	if err != nil {
		return sshKey{}, err
	}

	return sshKey{
		data:        data,
		fingerprint: fp,
		name:        fmt.Sprintf("%s-%s", r.cfg.Infra.StackName, base64.RawURLEncoding.EncodeToString(fp)),
	}, nil
}

func collectOutput(cmd *exec.Cmd) ([]byte, error) {
	output, err := cmd.Output()
	if err == nil {
		return output, nil
	}

	var stderr string
	if ee := (*exec.ExitError)(nil); errors.As(err, &ee) {
		stderr = "\n" + string(ee.Stderr)
	}

	return nil, fmt.Errorf("%s: %w%s", cmd.String(), err, stderr)
}
