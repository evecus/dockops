package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	HTTPPort  int
	HTTPSPort int
	CertPath  string
	KeyPath   string
	DataPath  string
}

// New creates a Config from CLI flags.
// Zero values mean "use default".
func New(httpPort, httpsPort int, dataDir string) *Config {
	cfg := &Config{
		HTTPPort:  9080,
		HTTPSPort: 9443,
		DataPath:  defaultDataPath(),
	}
	if httpPort > 0 {
		cfg.HTTPPort = httpPort
	}
	if httpsPort > 0 {
		cfg.HTTPSPort = httpsPort
	}
	if dataDir != "" {
		cfg.DataPath = dataDir
	}
	return cfg
}

// Init creates required directories and auto-detects TLS certs.
// Cert files must be placed in <dataPath>/cert/:
//   - cert.pem  (or server.crt / fullchain.pem)
//   - key.pem   (or server.key / privkey.pem)
func (c *Config) Init() error {
	certDir := filepath.Join(c.DataPath, "cert")
	dirs := []string{c.DataPath, certDir, filepath.Join(c.DataPath, "compose")}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}

	// Auto-detect cert/key pairs
	certCandidates := []string{"cert.pem", "fullchain.pem", "server.crt"}
	keyCandidates := []string{"key.pem", "privkey.pem", "server.key"}

	for _, cert := range certCandidates {
		p := filepath.Join(certDir, cert)
		if _, err := os.Stat(p); err == nil {
			c.CertPath = p
			break
		}
	}
	for _, key := range keyCandidates {
		p := filepath.Join(certDir, key)
		if _, err := os.Stat(p); err == nil {
			c.KeyPath = p
			break
		}
	}
	// Only enable HTTPS if both files are found
	if c.CertPath == "" || c.KeyPath == "" {
		c.CertPath = ""
		c.KeyPath = ""
	}
	return nil
}

// defaultDataPath returns ./data relative to the current working directory.
func defaultDataPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "./data"
	}
	return filepath.Join(filepath.Dir(exe), "data")
}
