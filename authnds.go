package main

import (
	"crypto/tls"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/GeertJohan/yubigo"
	"github.com/docopt/docopt-go"
	"github.com/metala/ldap"
	"github.com/op/go-logging"
)

const programName = "authnds"

var usage = `authnds: simple LDAP for auth

Usage:
  authnds [options] -c <file|s3url>
  authnds -h --help
  authnds --version

Options:
  -c, --config <file>       Config file.
  -h, --help                Show this screen.
  --version                 Show version.
`

// interface for backend handler
type Backend interface {
	ldap.Binder
	ldap.Searcher
	ldap.Closer
}

var log = logging.MustGetLogger(programName)

func main() {
	stderr := initLogging()
	log.Debug("AP start")

	cfg, err := doConfig()
	if err != nil {
		log.Fatalf("Configuration file error: %s", err.Error())
	}
	if cfg.Syslog {
		enableSyslog(stderr)
	}

	yubiAuth := (*yubigo.YubiAuth)(nil)
	if len(cfg.YubikeyClientID) > 0 && len(cfg.YubikeySecret) > 0 {
		yubiAuth, err = yubigo.NewYubiAuth(cfg.YubikeyClientID, cfg.YubikeySecret)

		if err != nil {
			log.Fatalf("Yubikey Auth failed")
		}
	}

	// configure the backend
	s := ldap.NewServer()
	s.EnforceLDAP = true
	if cfg.LDAPS.EnforceTLS {
		s.EnforceTLS = true
		cert, err := tls.LoadX509KeyPair(cfg.LDAPS.Cert, cfg.LDAPS.Key)
		if err != nil {
			log.Fatalf("Unable to load TLS configuration.")
		}
		s.TLSConfig = &tls.Config{
			ServerName:   cfg.ServerName,
			Certificates: []tls.Certificate{cert},
		}

	}

	handler := newConfigHandler(cfg, yubiAuth)
	log.Notice("Using config backend")
	s.BindFunc("", handler)
	s.SearchFunc("", handler)
	s.CloseFunc("", handler)

	if cfg.LDAP.Enabled {
		// Dont block if also starting a LDAPS server afterwards
		shouldBlock := !cfg.LDAPS.Enabled

		if shouldBlock {
			startLDAP(&cfg.LDAP, s)
		} else {
			go startLDAP(&cfg.LDAP, s)
		}
	}

	if cfg.LDAPS.Enabled {
		// Always block here
		startLDAPS(&cfg.LDAPS, s)
	}

	log.Critical("AP exit")
}

func startLDAP(ldapConfig *configLDAP, server *ldap.Server) {
	log.Notice(fmt.Sprintf("LDAP server listening on %s", ldapConfig.Listen))
	if err := server.ListenAndServe(ldapConfig.Listen); err != nil {
		log.Fatalf("LDAP Server Failed: %s", err.Error())
	}
}

func startLDAPS(ldapsConfig *configLDAPS, server *ldap.Server) {
	log.Notice(fmt.Sprintf("LDAPS server listening on %s", ldapsConfig.Listen))
	if err := server.ListenAndServeTLS(ldapsConfig.Listen, ldapsConfig.Cert, ldapsConfig.Key); err != nil {
		log.Fatalf("LDAP Server Failed: %s", err.Error())
	}
}

// doConfig reads the cli flags and config file
func doConfig() (*config, error) {
	cfg := config{}
	// setup defaults
	cfg.LDAP.Enabled = false
	cfg.LDAPS.Enabled = true

	// parse the command-line args
	args, err := docopt.Parse(usage, nil, true, getVersionString(), false)
	if err != nil {
		return &cfg, err
	}

	// parse the config file
	if _, err := toml.DecodeFile(args["--config"].(string), &cfg); err != nil {
		return &cfg, err
	}
	// Setup logging level
	switch cfg.LogLevel {
	case "debug":
		logging.SetLevel(logging.DEBUG, programName)
		log.Debug("Debugging enabled")
	case "error":
		logging.SetLevel(logging.ERROR, programName)
	case "info":
		logging.SetLevel(logging.INFO, programName)
		log.Debug("Debugging enabled")
	case "warning":
		logging.SetLevel(logging.WARNING, programName)
	default:
		logging.SetLevel(logging.NOTICE, programName)
	}

	if len(cfg.Frontend.Listen) > 0 && (len(cfg.LDAP.Listen) > 0 || len(cfg.LDAPS.Listen) > 0) {
		// Both old server-config and new - dont allow
		return &cfg, fmt.Errorf("Both old and new server-config in use - please remove old format ([frontend]) and migrate to new format ([ldap], [ldaps])")
	}

	if len(cfg.Frontend.Listen) > 0 {
		// We're going with old format - parse it into new
		log.Warning("Config [frontend] is deprecated - please move to [ldap] and [ldaps] as-per documentation")

		cfg.LDAP.Enabled = !cfg.Frontend.TLS
		cfg.LDAPS.Enabled = cfg.Frontend.TLS

		if cfg.Frontend.TLS {
			cfg.LDAPS.Listen = cfg.Frontend.Listen
		} else {
			cfg.LDAP.Listen = cfg.Frontend.Listen
		}

		if len(cfg.Frontend.Cert) > 0 {
			cfg.LDAPS.Cert = cfg.Frontend.Cert
		}
		if len(cfg.Frontend.Key) > 0 {
			cfg.LDAPS.Key = cfg.Frontend.Key
		}
	}

	if !cfg.LDAP.Enabled && !cfg.LDAPS.Enabled {
		return &cfg, fmt.Errorf("No server configuration found: please provide either LDAP or LDAPS configuration")
	}

	if cfg.LDAPS.Enabled {
		// LDAPS enabled - verify requirements (cert, key, listen)
		if len(cfg.LDAPS.Cert) == 0 || len(cfg.LDAPS.Key) == 0 {
			return &cfg, fmt.Errorf("LDAPS was enabled but no certificate or key were specified: please disable LDAPS or use the 'cert' and 'key' options")
		}

		if len(cfg.LDAPS.Listen) == 0 {
			return &cfg, fmt.Errorf("No LDAPS bind address was specified: please disable LDAPS or use the 'listen' option")
		}
	}

	if cfg.LDAP.Enabled {
		// LDAP enabled - verify listen
		if len(cfg.LDAP.Listen) == 0 {
			return &cfg, fmt.Errorf("No LDAP bind address was specified: please disable LDAP or use the 'listen' option")
		}
	}

	return &cfg, nil
}

// initLogging sets up logging to stderr
func initLogging() *logging.LogBackend {
	format := "%{color}%{time:15:04:05.000000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}"
	logBackend := logging.NewLogBackend(os.Stderr, "", 0)
	logging.SetBackend(logBackend)
	logging.SetLevel(logging.NOTICE, programName)
	logging.SetFormatter(logging.MustStringFormatter(format))
	return logBackend
}

// enableSyslog turns on syslog and turns off color
func enableSyslog(stderrBackend *logging.LogBackend) {
	format := "%{time:15:04:05.000000} %{shortfunc} ▶ %{level:.4s} %{id:03x} %{message}"
	logging.SetFormatter(logging.MustStringFormatter(format))
	syslogBackend, err := logging.NewSyslogBackend("")
	if err != nil {
		log.Fatal(err)
	}
	logging.SetBackend(stderrBackend, syslogBackend)
	log.Debug("Syslog enabled")
}
