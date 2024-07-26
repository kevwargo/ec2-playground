package config

type RunConfig struct {
	Images           []string
	Type             string
	Name             string
	Tags             []string
	Profile          string
	Policies         []string
	KeyPair          string
	SSHPublicKeyFile string
	SkipPublicIPv4   bool
	UserData         string
	InfraStackName   string
	SkipInfraDeploy  bool
	DumpFormat       VMFormat
	DryRun           bool
}
