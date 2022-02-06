package main

// config file
type configBackend struct {
	BaseDN   string
	Insecure bool     // For LDAP backend only
	Servers  []string // For LDAP backend only
}
type configFrontend struct {
	AllowedBaseDNs []string // For LDAP backend only
	Listen         string
	Cert           string
	Key            string
	TLS            bool
}
type configLDAP struct {
	Enabled bool
	Listen  string
}
type configLDAPS struct {
	Enabled    bool
	Listen     string
	Cert       string
	Key        string
	EnforceTLS bool
}
type configUser struct {
	CommonName string
	Disabled   bool
	// Person
	DisplayName  string
	GivenName    string
	Surname      string
	UserPassword string
	Mail         string
	// Posix
	PosixGroupID int
	PosixUserID  int
	Homedir      string
	LoginShell   string
	SSHKeys      []string
	// 2FA
	OTPSecret string
	Yubikey   string
	// Extra
	GroupNames    []string
	PassAppSHA256 []string
}
type configGroup struct {
	CommonName  string
	Description string
}
type config struct {
	ServerName         string
	Backend            configBackend
	LogLevel           string
	YubikeyClientID    string
	YubikeySecret      string
	Frontend           configFrontend
	LDAP               configLDAP
	LDAPS              configLDAPS
	Groups             []configGroup
	Syslog             bool
	Users              []configUser
	ConfigFile         string
	AwsAccessKeyId     string
	AwsSecretAccessKey string
	AwsRegion          string
}
