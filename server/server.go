package server

////
//// Wrapper around standalone Nuage agent server.
////

import (
	"encoding/json"
	"fmt"
	"net/http"

	agent "github.com/OpenPlatformSDN/nuage-cni/agent/server"
	"github.com/OpenPlatformSDN/nuage-cni/config"
	"github.com/OpenPlatformSDN/nuage-cni/errors"
	vsdclient "github.com/OpenPlatformSDN/nuage-oci-agent/vsd-client"
	"github.com/golang/glog"
	"github.com/gorilla/mux"
	"github.com/nuagenetworks/go-bambou/bambou"
	"github.com/nuagenetworks/vspk-go/vspk"
)

// Wrapper function around the agent Server

func Server(conf config.AgentConfig) error {

	// Use the locally defined handler for Container PUT instead of the agent server default
	agent.PutContainer = putContainer

	return agent.Server(conf)

}

// Local handler for ContainerPUT.

func putContainer(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	newc := vspk.Container{}
	if err := json.NewDecoder(req.Body).Decode(&newc); err != nil {
		glog.Errorf("Container create request - JSON decoding error: %s", err)
		agent.Sendjson(w, bambou.NewBambouError(errors.ContainerCannotCreate+vars["name"], "JSON decoding error"), http.StatusBadRequest)
		return
	}

	// Validate Enterprise name in container metadata against local config
	if newc.EnterpriseName != vsdclient.Enterprise.Name {
		err := fmt.Errorf("Container create request error: Container metadata Enterprise Name: %s does not match local configuration", newc.EnterpriseName)
		glog.Error(err)
		return
	}

	glog.Infof("Validated Container metadata - Enterprise: %s", newc.EnterpriseName)

	// Validate Domain name (encoded in contaier "DomainIDs") vs local config
	if len(newc.DomainIDs) != 1 {
		glog.Error("Container create request error: No metadata for container Domain")
		return
	}

	if newc.DomainIDs[0].(string) != vsdclient.Domain.Name {
		glog.Errorf("Container create request error: Container metadata Domain Name: %s does not match local configuration", newc.DomainIDs[0].(string))
		return
	}
	glog.Infof("Validated Container metadata - Domain: %s", newc.DomainIDs[0].(string))
	// reset that field
	newc.DomainIDs = nil

	// Validate Zone name (encoded in contaier "ZoneIDs")
	if len(newc.ZoneIDs) != 1 {
		glog.Error("Container create request error: No metadata for container Zone")
		return
	}

	if vsdclient.GetZone(newc.ZoneIDs[0].(string)) == nil {
		glog.Errorf("Container create request error: Container metadata Zone Name: %s does not match local configuration", newc.ZoneIDs[0].(string))
		return
	}
	glog.Infof("Validated Container metadata - Zone: %s", newc.ZoneIDs[0].(string))
	// reset that field
	newc.ZoneIDs = nil

	// Validate Subnet name (encoded in contaier "SubnetIDs")
	if len(newc.SubnetIDs) != 1 {
		glog.Error("Container create request error: No metadata for container Subnet")
		return
	}

	if vsdclient.GetSubnet(newc.SubnetIDs[0].(string)) == nil {
		glog.Errorf("Container create request error: Container metadata Subnet Name: %s does not match local configuration", newc.SubnetIDs[0].(string))
		return
	}
	glog.Infof("Validated Container metadata - Subnet: %s", newc.SubnetIDs[0].(string))
	// reset that field
	newc.SubnetIDs = nil

	//
	////
	////  ...Any additional processing at Container caching
	////

	agent.Containers[newc.Name] = newc

	////
	//// Response ....
	////

	glog.Infof("Successfully cached Nuage Container: %s", newc.Name)
	agent.Sendjson(w, nil, http.StatusCreated)
}
