package config

import (
	"io/ioutil"

	nuagecni "github.com/OpenPlatformSDN/nuage-cni/config"

	yaml "gopkg.in/yaml.v2"
)

// nuage-oci-agent server -- configuration file
type Config struct {
	// Not supplied in YAML config file
	ConfigFile string `yaml:"-"`
	// Config file fields
	Vsd         vsdConfig            `yaml:"vsd-config"`
	AgentServer nuagecni.AgentConfig `yaml:"agent-config"`
}

type vsdConfig struct {
	Url        string `yaml:"url"`
	APIVersion string `yaml:"apiversion"`
	Enterprise string `yaml:"enterprise"`
	Domain     string `yaml:"domain"`
	CertFile   string `yaml:"certFile"`
	KeyFile    string `yaml:"keyFile"`
}

func LoadConfig(conf *Config) error {
	data, err := ioutil.ReadFile(conf.ConfigFile)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, conf); err != nil {
		return err
	}

	return nil
}
