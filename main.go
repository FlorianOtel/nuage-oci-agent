package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/OpenPlatformSDN/nuage-oci-agent/server"

	"github.com/golang/glog"

	"github.com/OpenPlatformSDN/nuage-oci-agent/config"

	vsdclient "github.com/OpenPlatformSDN/nuage-oci-agent/vsd-client"
)

const errorLogLevel = "INFO"

var (
	// Top level Agent Server Configuration
	Config = new(config.Config)

	UseNetPolicies = false
)

////////
////////
////////

func init() {

	flag.CommandLine.StringVar(&Config.ConfigFile, "config",
		"./nuage-oci-agent-config.yaml", "configuration file for Nuage OCI agent server. If this file is specified, all remaining arguments will be ignored")

	// VSD flags
	flag.CommandLine.StringVar(&Config.Vsd.Url, "vsdurl",
		"", "Nuage VSD URL")
	flag.CommandLine.StringVar(&Config.Vsd.APIVersion, "vsdapiversion",
		"v5_0", "Nuage VSP API Version")
	flag.CommandLine.StringVar(&Config.Vsd.Enterprise, "vsdenterprise",
		"", "Nuage Enterprise Name for OCI containers")
	flag.CommandLine.StringVar(&Config.Vsd.Domain, "vsddomain",
		"", "Nuage Domain Name for OCI containers")
	flag.CommandLine.StringVar(&Config.Vsd.CertFile, "vsdcertfile",
		"./nuage-oci-agent-server.crt", "VSD login certificate file")
	flag.CommandLine.StringVar(&Config.Vsd.KeyFile, "vsdkeyfile",
		"./nuage-oci-agent-server.key", "VSD login private key file")

	// Agent Server flags
	flag.CommandLine.StringVar(&Config.AgentServer.ServerPort, "serverport",
		"7443", "Server port")

	flag.CommandLine.StringVar(&Config.AgentServer.CaFile, "cafile",
		"/opt/nuage/etc/ca.crt", "Server CA certificate")

	flag.CommandLine.StringVar(&Config.AgentServer.CertCaFile, "certcafile",
		"/opt/nuage/etc/agent-server.pem", "Server certificate (server + CA certificates PEM file)")

	flag.CommandLine.StringVar(&Config.AgentServer.KeyFile, "keyfile",
		"/opt/nuage/etc/agent-server.key", "Server private key file")

	// Set the values for log_dir and logtostderr.  Because this happens before flag.Parse(), cli arguments will override these.
	// Also set the DefValue parameter so -help shows the new defaults.
	// XXX - Make sure "glog" package is imported at this point, otherwise this will panic
	flag.CommandLine.Lookup("log_dir").DefValue = fmt.Sprintf("/var/log/%s", path.Base(os.Args[0]))
	flag.CommandLine.Lookup("logtostderr").DefValue = "false"
	flag.CommandLine.Lookup("stderrthreshold").DefValue = errorLogLevel

	flag.Parse()

	// Set log_dir -- either to given value or to the default + create the directory
	if mylogdir := flag.CommandLine.Lookup("log_dir").Value.String(); mylogdir != "" {
		os.MkdirAll(mylogdir, os.ModePerm)
	} else { // set it to default log_dir value
		flag.CommandLine.Lookup("log_dir").Value.Set(flag.CommandLine.Lookup("log_dir").DefValue)
		os.MkdirAll(flag.CommandLine.Lookup("log_dir").DefValue, os.ModePerm)
	}

	// Periodic flush of glog logs
	go func() {
		// Adjust accordingly
		var interval = 1 * time.Second

		for _ = range time.Tick(interval) {
			glog.Flush()
		}
	}()

}

// Silly little wrapper around os.Exit(). Needed since os.Exit() does not honor defer calls and glog.Fatalf() looks ugly _and_ does not flush the logs.
func osExit(context string, err error) {
	glog.Errorf("%s: %s", context, err)
	glog.Flush()
	os.Exit(255)
}

func main() {

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

	if err := server.Server(Config.AgentServer); err != nil {
		osExit("Failed to start OCI agent server", err)
	}

}
