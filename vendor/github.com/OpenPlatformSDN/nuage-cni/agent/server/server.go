package server

import (
	"encoding/json"
	"net/http"

	"github.com/OpenPlatformSDN/nuage-cni/agent/types"
	nuagecnitypes "github.com/OpenPlatformSDN/nuage-cni/types"

	"github.com/OpenPlatformSDN/nuage-cni/config"
	"github.com/OpenPlatformSDN/nuage-cni/errors"
	"github.com/nuagenetworks/vspk-go/vspk"

	"github.com/nuagenetworks/go-bambou/bambou"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
)

var (
	// Nuage Containers cache -- containers running on this node
	// Key: vspk.Container.Name
	// - For K8S: <podName>_<podNs>.
	// - For runc: Container ID (Note: For cri-o containers the runc ID is actually an UUID, not  a name)
	Containers = make(map[string]vspk.Container)

	// Subnets with endpoints on the local node
	// XXX -- This cache is NOT necessarily consistent with the information in the VSD
	// Key: CNI network name  <-> vspk.Subnet.Name
	Networks = make(map[string]nuagecnitypes.NetConf)

	// Interfaces of containers running on this host
	// XXX - Since Nuage containers may have each interface, and each interface is part of a single 'Result', a container corresponds to []Result
	// Key:  vspk.Container.Name
	Interfaces = make(map[string][]nuagecnitypes.Result)
)

// We allow the redefinition of those functions by server wrappers. Otherwise we use the default handlers (below)
var (
	// Networks

	PostNetwork   func(http.ResponseWriter, *http.Request) = postNetwork
	GetNetworks   func(http.ResponseWriter, *http.Request) = getNetworks
	GetNetwork    func(http.ResponseWriter, *http.Request) = getNetwork
	DeleteNetwork func(http.ResponseWriter, *http.Request) = deleteNetwork

	// Containers

	PutContainer    func(http.ResponseWriter, *http.Request) = putContainer
	GetContainers   func(http.ResponseWriter, *http.Request) = getContainers
	GetContainer    func(http.ResponseWriter, *http.Request) = getContainer
	DeleteContainer func(http.ResponseWriter, *http.Request) = deleteContainer

	// Container Interfaces

	PutContainerInterfaces    func(http.ResponseWriter, *http.Request) = putContainerInterfaces
	GetInterfaces             func(http.ResponseWriter, *http.Request) = getInterfaces
	GetContainerInterfaces    func(http.ResponseWriter, *http.Request) = getContainerInterfaces
	DeleteContainerInterfaces func(http.ResponseWriter, *http.Request) = deleteContainerInterfaces
)

func Server(conf config.AgentConfig) error {

	router := mux.NewRouter()

	////
	//// CNI Networks: Create/Retrieve/Delete CNI NetConf
	////
	// POST <-- NetConf
	router.HandleFunc(types.NetconfPath, PostNetwork).Methods("POST")
	// GET   --> NetConf
	router.HandleFunc(types.NetconfPath, GetNetworks).Methods("GET")
	router.HandleFunc(types.NetconfPath+"{name}", GetNetwork).Methods("GET")
	// DELETE <-- NetConf
	router.HandleFunc(types.NetconfPath+"{name}", DeleteNetwork).Methods("DELETE")

	////
	//// Cached Containers: Cache / retrieve vspk.Container (temporary cache; Contains entries / valid during the top part of split activation). Only PUT, GET, DELETE.
	////
	// PUT  <-- vspk.Container
	router.HandleFunc(types.ContainerPath+"{name}", PutContainer).Methods("PUT")
	// GET --> vspk.Container
	router.HandleFunc(types.ContainerPath, GetContainers).Methods("GET")
	router.HandleFunc(types.ContainerPath+"{name}", GetContainer).Methods("GET")
	// DELETE --> vspk.Container
	router.HandleFunc(types.ContainerPath+"{name}", DeleteContainer).Methods("DELETE")

	////
	////  CNI Interfaces: Create/Modify/Retreive/Delete []Result
	////  - Only PUT with a specific Name. "Name" convention may be specific to a platform. E.g. for K8S is <podName>_<podNameSpace>
	////
	// PUT
	router.HandleFunc(types.ResultPath+"{name}", PutContainerInterfaces).Methods("PUT")
	// GET --> Result
	router.HandleFunc(types.ResultPath, GetInterfaces).Methods("GET")
	router.HandleFunc(types.ResultPath+"{name}", GetContainerInterfaces).Methods("GET")
	// DELETE <-- uuid
	router.HandleFunc(types.ResultPath+"{name}", DeleteContainerInterfaces).Methods("DELETE")

	////
	////
	////
	return http.ListenAndServeTLS(":"+conf.ServerPort, conf.CertCaFile, conf.KeyFile, router)
}

////////
//////// Util
////////

func Sendjson(w http.ResponseWriter, data interface{}, httpstatus int) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(httpstatus)
	json.NewEncoder(w).Encode(data)
}

////////
//////// Handlers. They can be re-defined by wrappers
////////

////
//// Networks
////

func postNetwork(w http.ResponseWriter, req *http.Request) {
	netconf := nuagecnitypes.NetConf{}
	if err := json.NewDecoder(req.Body).Decode(&netconf); err != nil {
		glog.Errorf("Network Configuration create request - JSON decoding error: %s", err)
		Sendjson(w, bambou.NewBambouError(errors.NetworkCannotCreate, "JSON decoding error"), http.StatusBadRequest)
		return
	}

	if netconf.NetConf.Name == "" {
		glog.Warningf("Cannot create CNI Network Configuration with an empty name: %#v", netconf.NetConf)
		Sendjson(w, bambou.NewBambouError(errors.NetworkCannotCreate, "Network Configuration lacks a valid name"), http.StatusBadRequest)
		return
	}

	if _, exists := Networks[netconf.NetConf.Name]; exists {
		glog.Warningf("Cannot create CNI Network Configuration with dulicate name: %s", netconf.NetConf.Name)
		Sendjson(w, bambou.NewBambouError(errors.NetworkCannotCreate+netconf.NetConf.Name, "Network Configuration already exists"), http.StatusConflict)
		return
	}

	////
	////  ...Any additional processing at network creation
	//// - Scrubbing (?)

	Networks[netconf.NetConf.Name] = netconf

	////
	//// Response ....
	////

	glog.Infof("Successfully created CNI Network Configuration named: %s", netconf.NetConf.Name)
	Sendjson(w, nil, http.StatusCreated)
}

func getNetworks(w http.ResponseWriter, req *http.Request) {
	glog.Infof("Serving the list of local CNI Network Configurations")
	var resp []nuagecnitypes.NetConf
	for _, netw := range Networks {
		resp = append(resp, netw)
	}
	Sendjson(w, resp, http.StatusOK)
}

func getNetwork(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	if mynetw, exists := Networks[vars["name"]]; exists {
		glog.Infof("Serving CNI Network Configuration: %s", mynetw.Name)
		Sendjson(w, mynetw, http.StatusOK)
	} else {
		glog.Warningf("Cannot find CNI Network Configuration: %s", vars["name"])
		Sendjson(w, bambou.NewBambouError(errors.NetworkNotFound+vars["name"], ""), http.StatusNotFound)
	}
}

func deleteNetwork(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	if mynetw, exists := Networks[vars["name"]]; exists {
		glog.Infof("Deleting CNI Network Configuration: %s", mynetw.Name)
		////
		//// ... Any additional processing when deleting a network
		////
		delete(Networks, vars["name"])
	} else {
		glog.Warningf("Cannot delete CNI Network Configuration: %s", vars["name"])
		Sendjson(w, bambou.NewBambouError(errors.NetworkNotFound+vars["name"], ""), http.StatusNotFound)
	}
}

////
//// Containers
////

// Put container in local cache at specfic URI
// XXX - Notes
// - Different than "canonical" PUT -- i.e. no modify, only create at specific URI
// - Allow overwrites
func putContainer(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	/*
		if _, exists := Containers[vars["name"]]; exists {
			glog.Warningf("Cannot cache Nuage Container with duplicate Name: %s", vars["name"])
			Sendjson(w, bambou.NewBambouError(errors.ContainerCannotCreate+vars["name"], "A Nuge Container with given name already exists"), http.StatusConflict)
			return
		}
	*/

	newc := vspk.Container{}
	if err := json.NewDecoder(req.Body).Decode(&newc); err != nil {
		glog.Errorf("Container create request - JSON decoding error: %s", err)
		Sendjson(w, bambou.NewBambouError(errors.ContainerCannotCreate+vars["name"], "JSON decoding error"), http.StatusBadRequest)
		return
	}

	////
	////  ...Any additional processing at Container caching
	////

	Containers[newc.Name] = newc

	////
	//// Response ....
	////

	glog.Infof("Successfully cached Nuage Container: %s", newc.Name)
	Sendjson(w, nil, http.StatusCreated)
}

// List all cached containers
func getContainers(w http.ResponseWriter, req *http.Request) {
	glog.Info("Serving list of currently cached Nuage Containers")
	var resp []vspk.Container
	for _, container := range Containers {
		resp = append(resp, container)
	}
	Sendjson(w, resp, http.StatusOK)
}

// Get container with given Name
func getContainer(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	if container, exists := Containers[vars["name"]]; exists {
		glog.Infof("Serving Nuage Container: %s", container.Name)
		Sendjson(w, container, http.StatusOK)
	} else {
		glog.Warningf("Cannot find Nuage Container: %s", vars["name"])
		Sendjson(w, bambou.NewBambouError(errors.ContainerNotFound+vars["name"], ""), http.StatusNotFound)
	}

}

// Delete container from cache
func deleteContainer(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	if container, exists := Containers[vars["name"]]; exists {
		glog.Infof("Deleting cached Nuage Container: %s", container.Name)
		////
		//// ... Any additional processing when deleting a container
		////
		delete(Containers, vars["name"])
	} else {
		glog.Warningf("Cannot find Nuage Container: %s", vars["name"])
		Sendjson(w, bambou.NewBambouError(errors.ContainerNotFound+vars["name"], ""), http.StatusNotFound)
	}

}

////
////  CNI Interfaces: Interfaces (running containers) in CNI Result format
////

// Create/Update Container interface information for a specific container Name
// XXX - Allow overwrites
func putContainerInterfaces(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	/*
		if _, exists := Interfaces[vars["name"]]; exists {
			glog.Warningf("Container Interface information already exists for Container with Name: %s", vars["name"])
			Sendjson(w, bambou.NewBambouError(errors.ContainerCannotCreate+vars["name"], "Container Interfaces already created"), http.StatusConflict)
			return
		}
	*/

	var containerifaces []nuagecnitypes.Result
	if err := json.NewDecoder(req.Body).Decode(&containerifaces); err != nil {
		glog.Errorf("Container Interfaces modify request - JSON decoding error: %s", err)
		Sendjson(w, bambou.NewBambouError(errors.ContainerCannotModify+vars["name"], "JSON decoding error"), http.StatusBadRequest)
		return
	}

	////
	////  ...Any additional processing at container interface modification
	//// - (Scrubbing ?)

	Interfaces[vars["name"]] = containerifaces

	////
	//// Response ....
	////

	glog.Infof("Successfully modified CNI Interfaces configuration for Container: %s", vars["name"])
	Sendjson(w, nil, http.StatusOK)
}

// Get all interfaces for all running containers
func getInterfaces(w http.ResponseWriter, req *http.Request) {
	glog.Infof("Serving list of current CNI container interface information in CNI Result format")
	var resp [][]nuagecnitypes.Result
	for _, rez := range Interfaces {
		resp = append(resp, rez)
	}
	Sendjson(w, resp, http.StatusOK)
}

// Get all interfaces of a given container
func getContainerInterfaces(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	if cifaces, exists := Interfaces[vars["name"]]; exists {
		glog.Infof("Serving in CNI Result format the interfaces for Container: %s", vars["name"])
		Sendjson(w, cifaces, http.StatusOK)
	} else {
		glog.Warningf("Cannot find CNI interface information for Container: %s", vars["name"])
		Sendjson(w, bambou.NewBambouError(errors.ContainerNotFound+vars["name"], ""), http.StatusNotFound)
	}
}

// Delete all interfaces of a given container
func deleteContainerInterfaces(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	if _, exists := Interfaces[vars["name"]]; exists {
		glog.Infof("Deleting CNI interface information for Container: %s", vars["name"])
		delete(Interfaces, vars["name"])
	} else {
		glog.Warningf("Cannot delete CNI interface information for Container: %s", vars["name"])
		Sendjson(w, bambou.NewBambouError(errors.ContainerNotFound+vars["name"], ""), http.StatusNotFound)
	}
}
