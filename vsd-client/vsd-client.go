package vsdclient

import (
	"crypto/tls"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"

	"github.com/OpenPlatformSDN/nuage-oci/config"

	"github.com/nuagenetworks/go-bambou/bambou"
	"github.com/nuagenetworks/vspk-go/vspk"
)

const (
	MAX_SUBNETS = 2048 // Practical, safety max limit on nr Subnets we handle (upper limit for 1<< SubnetLength)
)

var (
	// Nuage API connection defaults. We need to keep them as global vars since commands can be invoked in whatever order.

	root      *vspk.Me
	mysession *bambou.Session

	// Nuage Enterprise and Domain for OCI containers. They must exist.
	Enterprise *vspk.Enterprise
	Domain     *vspk.Domain

	// Serialize VSD operations, esp creates/updates
	vsdmutex sync.Mutex
)

func InitClient(conf *config.Config) error {

	if err := makeX509conn(conf); err != nil {
		return bambou.NewBambouError("Nuage TLS API connection failed", err.Error())
	}

	if conf.VsdConfig.Enterprise == "" || conf.VsdConfig.Domain == "" {
		return bambou.NewBambouError("Nuage VSD Enterprise and/or Domain are absent from configuration file", "")
	}

	//// Find the  Enterprise and Domain. They must be pre-existing in the VSD.

	//// VSD Enterprise
	if el, err := root.Enterprises(&bambou.FetchingInfo{Filter: "name == \"" + conf.VsdConfig.Enterprise + "\""}); err != nil {
		return bambou.NewBambouError("Error fetching list of Enterprises from the VSD", err.Error())
	} else {
		if len(el) != 1 { // Given Enterprise doesn't exist
			return bambou.NewBambouError("Cannot find VSD Enterprise: "+conf.VsdConfig.Enterprise, "VSD Enterprise not found")
		}

		Enterprise = el[0]
		glog.Infof("Found existing Enterprise: %#s", Enterprise.Name)
	}

	////  VSD Domain
	if dl, err := root.Domains(&bambou.FetchingInfo{Filter: "name == \"" + conf.VsdConfig.Domain + "\""}); err != nil {
		return bambou.NewBambouError("Error fetching list of Domains from the VSD", err.Error())
	} else {
		if len(dl) != 1 {
			return bambou.NewBambouError("Cannot find VSD Domain: "+conf.VsdConfig.Domain, "VSD Domain not found")
		}

		Domain = dl[0]
		glog.Infof("Found existing Domain: %#s", Domain.Name)
	}

	glog.Info("VSD client initialization completed")
	return nil
}

func GenerateMAC() string {
	buf := make([]byte, 6)
	rand.Seed(time.Now().UTC().UnixNano())
	rand.Read(buf)
	// Set the local bit -- Note the setting of the local bit which means it won't clash with any globally administered addresses (see wikipedia for more info)
	// XXX -- This does _NOT_ work for Nuage VSD
	// buf[0] |= 2
	// XXX - For Nuage VSD
	buf[0] = buf[0]&0xFE | 0x02
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", buf[0], buf[1], buf[2], buf[3], buf[4], buf[5])
}

////////
//////// utils
////////

// Create a connection to the VSD using X.509 certificate-based authentication
func makeX509conn(conf *config.Config) error {
	if cert, err := tls.LoadX509KeyPair(conf.VsdConfig.CertFile, conf.VsdConfig.KeyFile); err != nil {
		return err
	} else {
		mysession, root = vspk.NewX509Session(&cert, conf.VsdConfig.Url)
	}

	// mysession.SetInsecureSkipVerify(true)

	if err := mysession.Start(); err != nil {
		return err
	}

	glog.Infof("vsd-client: Successfully established a connection to the VSD at URL is: %s\n", conf.VsdConfig.Url)

	// glog.Infof("vsd-client: Successfuly established bambou session: %#v\n", *mysession)

	return nil
}

// XXX - Due to VSD create operations delays, simultaneous create operations may fail with "already exists" (particularly at startup).
// Here we check if the underlying error contains that string (as all "go-bambou" errors of this type should)

func alreadyexistserr(err error) bool {
	if be, ok := err.(*bambou.Error); ok {
		return strings.Contains(be.Description, "already exists")
	}
	return false
}
