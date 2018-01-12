package main

import (
	"flag"
	"fmt"
	"os"
	"path"

	"github.com/golang/glog"

	"github.com/OpenPlatformSDN/nuage-oci-agent/config"

	agentserver "github.com/OpenPlatformSDN/nuage-oci-agent/server"
	vsdclient "github.com/OpenPlatformSDN/nuage-oci-agent/vsd-client"
)

const errorLogLevel = "WARNING"

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
	flagSet.Lookup("log_dir").DefValue = fmt.Sprintf("/var/log/%s", path.Base(os.Args[0]))
	flagSet.Lookup("logtostderr").DefValue = "false"
	flagSet.Lookup("stderrthreshold").DefValue = errorLogLevel

	flag.Parse()

	// Set log_dir -- either to given value or to the default + create the directory
	if mylogdir := flag.CommandLine.Lookup("log_dir").Value.String(); mylogdir != "" {
		os.MkdirAll(mylogdir, os.ModePerm)
	} else { // set it to default log_dir value
		flag.CommandLine.Lookup("log_dir").Value.Set(flag.CommandLine.Lookup("log_dir").DefValue)
		os.MkdirAll(flag.CommandLine.Lookup("log_dir").DefValue, os.ModePerm)
	}

}

// Silly little wrapper around os.Exit(). Needed since os.Exit() does not honor defer calls and glog.Fatalf() looks ugly _and_ does not flush the logs.
func osExit(context string, err error) {
	glog.Errorf("%s: %s", context, err)
	glog.Flush()
	os.Exit(255)
}

func main() {

	Config = new(config.Config)

	Flags(Config, flag.CommandLine)

	if len(os.Args) == 1 { // With no arguments, print default usage
		flag.PrintDefaults()
		os.Exit(0)
	}
	// Flush the logs upon exit
	defer glog.Flush()

	glog.Infof("===> Starting %s...", path.Base(os.Args[0]))

	if err := config.LoadConfig(Config); err != nil {
		osExit("Cannot read configuration file", err)
	}

	if err := vsdclient.InitClient(Config); err != nil {
		osExit("VSD client error", err)
	}

	if err := agentserver.Server(Config.AgentServerConfig); err != nil {
		osExit("Failed to start OCI agent server", err)
	}

}
