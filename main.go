package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"

	"github.com/golang/glog"

	cniagent "github.com/OpenPlatformSDN/nuage-cni/agent/server"
	"github.com/OpenPlatformSDN/nuage-oci-agent/config"
	vsdclient "github.com/OpenPlatformSDN/nuage-oci-agent/vsd-client"
)

const errorLogLevel = 2

var (
	// Top level Agent Server Configuration
	Config         *config.Config
	UseNetPolicies = false
)

////////
////////
////////

func Flags(conf *config.Config, flagSet *flag.FlagSet) {
	flagSet.StringVar(&conf.ConfigFile, "config",
		"./nuage-oci-agent-config.yaml", "configuration file for Nuage OCI agent server. If this file is specified, all remaining arguments will be ignored")

	// VSD flags
	flagSet.StringVar(&conf.VsdConfig.Url, "vsdurl",
		"", "Nuage VSD URL")
	flagSet.StringVar(&conf.VsdConfig.APIVersion, "vsdapiversion",
		"v5_0", "Nuage VSP API Version")
	flagSet.StringVar(&conf.VsdConfig.Enterprise, "vsdenterprise",
		"", "Nuage Enterprise Name for OCI containers")
	flagSet.StringVar(&conf.VsdConfig.Domain, "vsddomain",
		"", "Nuage Domain Name for OCI containers")
	flagSet.StringVar(&conf.VsdConfig.CertFile, "vsdcertfile",
		"./nuage-oci-agent-server.crt", "VSD certificate file for Nuage OCI agent server")
	flagSet.StringVar(&conf.VsdConfig.KeyFile, "vsdkeyfile",
		"./nuage-oci-agent-server.key", "VSD private key file for Nuage OCI agent server")

	// Agent Server flags
	flagSet.StringVar(&conf.AgentServerConfig.ServerPort, "serverport",
		"7443", "Nuage OCI agent server port")

	flagSet.StringVar(&conf.AgentServerConfig.CaFile, "cafile",
		"/opt/nuage/etc/ca.crt", "Nuage OCI agent server CA certificate")

	flagSet.StringVar(&conf.AgentServerConfig.CertCaFile, "certcafile",
		"/opt/nuage/etc/agent-server.pem", "Nuage OCI agent server certificate (server + CA certificates PEM file)")

	flagSet.StringVar(&conf.AgentServerConfig.KeyFile, "keyfile",
		"/opt/nuage/etc/agent-server.key", "Nuage OCI agent server private key file")

	// Set the values for log_dir and logtostderr.  Because this happens before flag.Parse(), cli arguments will override these.
	// Also set the DefValue parameter so -help shows the new defaults.
	// XXX - Make sure "glog" package is imported at this point, otherwise this will panic
	log_dir := flagSet.Lookup("log_dir")
	log_dir.Value.Set(fmt.Sprintf("/var/log/%s", path.Base(os.Args[0])))
	log_dir.DefValue = fmt.Sprintf("/var/log/%s", path.Base(os.Args[0]))
	logtostderr := flagSet.Lookup("logtostderr")
	logtostderr.Value.Set("false")
	logtostderr.DefValue = "false"
	stderrlogthreshold := flagSet.Lookup("stderrthreshold")
	stderrlogthreshold.Value.Set("2")
	stderrlogthreshold.DefValue = "2"
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func main() {

	Config = new(config.Config)

	Flags(Config, flag.CommandLine)
	flag.Parse()

	if len(os.Args) == 1 { // With no arguments, print default usage
		flag.PrintDefaults()
		os.Exit(0)
	}
	// Flush the logs upon exit
	defer glog.Flush()

	glog.Infof("===> Starting %s...", path.Base(os.Args[0]))

	if err := config.LoadConfig(Config); err != nil {
		glog.Errorf("Cannot read configuration file: %s", err)
		os.Exit(255)
	}

	if err := vsdclient.InitClient(Config); err != nil {
		glog.Errorf("VSD client error: %s", err)
		os.Exit(255)
	}

	if err := cniagent.Server(Config.AgentServerConfig); err != nil {
		glog.Fatalf("Failed to start OCI agent server: %s", err)
	}

}
