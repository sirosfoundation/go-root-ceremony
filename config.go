package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// CeremonyType identifies the kind of ceremony being run.
type CeremonyType string

const (
	CeremonyRootCAWrap   CeremonyType = "root-ca-wrap"
	CeremonyRootCAKeygen CeremonyType = "root-ca-keygen"
	CeremonyIssuingWrap  CeremonyType = "issuing-wrap"
	CeremonyRecovery     CeremonyType = "recovery"
)

func (t CeremonyType) Title() string {
	switch t {
	case CeremonyRootCAWrap:
		return "Root CA — Wrap Key Generation"
	case CeremonyRootCAKeygen:
		return "Root CA — HSM Key Generation"
	case CeremonyIssuingWrap:
		return "Issuing CA — Wrap Key Generation"
	case CeremonyRecovery:
		return "Key Recovery Ceremony"
	default:
		return string(t)
	}
}

func (t CeremonyType) Description(n, m int) string {
	switch t {
	case CeremonyRootCAWrap:
		return fmt.Sprintf(
			"This ceremony establishes a new cryptographic wrap key for the Root CA signing "+
				"key stored in YubiHSM 2 FIPS. The wrap key is split across %d custodians using "+
				"Shamir Secret Sharing; any %d custodians must be present to reconstruct it.", n, m)
	case CeremonyRootCAKeygen:
		return "This ceremony generates the Root CA private key inside a YubiHSM 2 FIPS device " +
			"and establishes a wrap key for key backup and recovery."
	case CeremonyIssuingWrap:
		return fmt.Sprintf(
			"This ceremony establishes a new wrap key for the Issuing CA. The wrap key is "+
				"split across %d custodians; any %d must be present for reconstruction.", n, m)
	case CeremonyRecovery:
		return fmt.Sprintf(
			"This ceremony reconstructs a previously split secret using Shamir Secret Sharing. "+
				"At least %d custodians must be present with their physical YubiKeys.", m)
	default:
		return ""
	}
}

// StorageMethod describes how encrypted share files are stored after the ceremony.
type StorageMethod string

const (
	StorageUSB   StorageMethod = "usb"
	StoragePrint StorageMethod = "print"
	StorageBoth  StorageMethod = "both"
)

func (s StorageMethod) Note() string {
	switch s {
	case StoragePrint:
		return "Each encrypted share file is base64-encoded and printed as a QR code. " +
			"Custodians receive a printed sheet for off-site storage."
	case StorageBoth:
		return "Each encrypted share file is both written to a USB drive and printed as a " +
			"QR code. Custodians must store them separately."
	default:
		return "Each encrypted share file (custodianN.share.age) is written to a dedicated " +
			"USB drive and given to the respective custodian for secure off-site storage."
	}
}

// Person is a named ceremony participant.
type Person struct {
	Name string `yaml:"name"`
}

func (p Person) DisplayName(fallback string) string {
	if strings.TrimSpace(p.Name) != "" {
		return p.Name
	}
	return fallback
}

// ShamirConfig holds the SSS parameters.
type ShamirConfig struct {
	Shares    int `yaml:"shares"`
	Threshold int `yaml:"threshold"`
}

func (s ShamirConfig) Validate() error {
	if s.Shares < 3 || s.Shares > 10 {
		return fmt.Errorf("shares must be between 3 and 10, got %d", s.Shares)
	}
	if s.Threshold < 2 {
		return fmt.Errorf("threshold must be at least 2, got %d", s.Threshold)
	}
	if s.Threshold >= s.Shares {
		return fmt.Errorf("threshold (%d) must be less than shares (%d)", s.Threshold, s.Shares)
	}
	return nil
}

// Options holds optional ceremony steps.
type Options struct {
	IncludeVerification bool          `yaml:"include_verification"`
	IncludeYubiHSMImport bool         `yaml:"include_yubihsm_import"`
	ShareStorage        StorageMethod `yaml:"share_storage"`
}

// Config is the top-level ceremony configuration.
type Config struct {
	Organization string       `yaml:"organization"`
	CAName       string       `yaml:"ca_name"`
	Location     string       `yaml:"location"`
	Date         string       `yaml:"date"`
	Operator     string       `yaml:"operator"`
	CeremonyType CeremonyType `yaml:"ceremony_type"`
	Shamir       ShamirConfig `yaml:"shamir"`
	Custodians   []Person     `yaml:"custodians"`
	Witnesses    []Person     `yaml:"witnesses"`
	Options      Options      `yaml:"options"`
}

// DefaultConfig returns sensible defaults for a new ceremony config.
func DefaultConfig() Config {
	return Config{
		Organization: "",
		CAName:       "",
		Location:     "",
		Date:         time.Now().Format("2006-01-02"),
		Operator:     "",
		CeremonyType: CeremonyRootCAWrap,
		Shamir: ShamirConfig{
			Shares:    5,
			Threshold: 3,
		},
		Custodians: make([]Person, 5),
		Witnesses:  make([]Person, 2),
		Options: Options{
			IncludeVerification:  true,
			IncludeYubiHSMImport: true,
			ShareStorage:         StorageUSB,
		},
	}
}

// LoadConfig reads and parses a YAML config file.
func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("reading config file: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config file: %w", err)
	}
	return cfg, nil
}

// Validate checks the config is consistent and complete enough to generate a script.
func (c *Config) Validate() error {
	if err := c.Shamir.Validate(); err != nil {
		return err
	}
	if len(c.Custodians) != c.Shamir.Shares {
		// Pad or trim to match shares
		for len(c.Custodians) < c.Shamir.Shares {
			c.Custodians = append(c.Custodians, Person{})
		}
		c.Custodians = c.Custodians[:c.Shamir.Shares]
	}
	if len(c.Witnesses) < 2 {
		for len(c.Witnesses) < 2 {
			c.Witnesses = append(c.Witnesses, Person{})
		}
	}
	return nil
}

// CustodianName returns the display name for custodian i (1-indexed in display, 0-indexed here).
func (c *Config) CustodianName(i int) string {
	if i < len(c.Custodians) {
		return c.Custodians[i].DisplayName(fmt.Sprintf("Custodian %d", i+1))
	}
	return fmt.Sprintf("Custodian %d", i+1)
}

// WitnessName returns the display name for witness i.
func (c *Config) WitnessName(i int) string {
	if i < len(c.Witnesses) {
		return c.Witnesses[i].DisplayName(fmt.Sprintf("Witness %d", i+1))
	}
	return fmt.Sprintf("Witness %d", i+1)
}

func (c *Config) OrgDisplay() string {
	if c.Organization != "" {
		return c.Organization
	}
	return "[Organisation Name]"
}

func (c *Config) CADisplay() string {
	if c.CAName != "" {
		return c.CAName
	}
	return "[CA Name]"
}

func (c *Config) LocationDisplay() string {
	if c.Location != "" {
		return c.Location
	}
	return "[Location]"
}

func (c *Config) OperatorDisplay() string {
	if c.Operator != "" {
		return c.Operator
	}
	return "[Operator Name]"
}
