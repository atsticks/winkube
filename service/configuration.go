// Copyright 2019 Anatole Tresch
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/winkube/service/netutil"
	util2 "github.com/winkube/util"
	"net"
	"os"
)

const WINKUBE_CONFIG_FILE = "winkube-config.json"

type NodeType int

const (
	UndefinedNode NodeType = iota
	Worker
	Master
	Controller
)

func (this NodeType) String() string {
	return [...]string{"UndefinedNode", "Worker", "Master", "Controller"}[this]
}
func NodeType_Values() []NodeType {
	return []NodeType{
		UndefinedNode,
		Worker,
		Master,
		Controller,
	}
}

type VMNetType int

const (
	UndefinedNetType VMNetType = iota
	NAT
	Bridged
)

func (this VMNetType) String() string {
	return [...]string{"UndefinedNetType", "NAT", "Bridged"}[this]
}
func NodeNetType_Values() []VMNetType {
	return []VMNetType{
		UndefinedNetType,
		NAT,
		Bridged,
	}
}

type LocalHostConfig struct {
	NetHostInterface string `validate:"required"`
	NetHostname      string `validate:"required"`
	NetHostIP        string `validate:"required"`
}

type NetConfig struct {
	NetMulticastEnabled bool
	NetUPnPPort         int `validate:"required"`
	MasterController    string
}

type ClusterControllerConnection struct {
	ClusterId          string `validate:"required"`
	ClusterCredentials string
	ControllerHost     string `validate:"required"`
}

type ClusterConfig struct {
	ClusterId            string `validate:"required"`
	ClusterCredentials   string
	ClusterPodCIDR       string `validate:"required"`
	ClusterServiceDomain string `validate:"required"`
	// The net integration of the Nodes with their hosts
	ClusterVMNet VMNetType `validate:"required"`
	// The internal network, if NAT is used,or the external node network, if Bridge networking is used.
	ClusterNetCIDR       string `validate:"required"`
	ClusterControlPlane  string
	ClusterMasterAddress string
	ClusterMasterApiPort int
	ClusterAllWorkers    []ClusterNodeConfig
	ClusterAllMasters    []ClusterNodeConfig
	ClusterToken         string
}

// The primary master, if existing.
func (this ClusterConfig) PrimaryMaster() *ClusterNodeConfig {
	if len(this.ClusterAllMasters) > 0 {
		return &this.ClusterAllMasters[0]
	}
	return nil
}

// All configured masters.
func (this ClusterConfig) AllMasters() []ClusterNodeConfig {
	return this.ClusterAllMasters
}

// All configured workers.
func (this ClusterConfig) AllWorkers() []ClusterNodeConfig {
	return this.ClusterAllWorkers
}

type ClusterNodeConfig struct {
	NodeName            string   `validate:"required"` // node
	NodeType            NodeType `validate:"required"`
	NodeNetType         VMNetType
	NodeAddress         string
	NodeAddressInternal string
	NodeMemory          int `validate:"required,gte=1028"` // 2048
	NodeCPU             int `validate:"required,gte=1"`    // 2
	IsJoiningNode       bool
	NodeBox             string `validate:"required"` // ubuntu/xenial64, centos/7
	NodeBoxVersion      string `validate:"required"` // 20180831.0.0
}

type SystemConfiguration struct {
	Id string `validate:"required", json:"id"`
	LocalHostConfig
	NetConfig
	ClusterLogin     *ClusterControllerConnection `json:"clusterLogin"`
	ControllerConfig *ClusterConfig               `json:"cluster"`
	MasterNode       *ClusterNodeConfig           `json:"master"`
	WorkerNode       *ClusterNodeConfig           `json:"worker"`
}

func (this SystemConfiguration) IsWorkerNode() bool {
	return this.WorkerNode != nil
}
func (this SystemConfiguration) IsMasterNode() bool {
	return this.MasterNode != nil
}
func (this SystemConfiguration) IsPrimaryMaster() bool {
	return this.MasterNode != nil && !this.MasterNode.IsJoiningNode
}
func (this SystemConfiguration) IsJoiningMaster() bool {
	return this.MasterNode != nil && this.MasterNode.IsJoiningNode
}
func (this SystemConfiguration) IsControllerNode() bool {
	return this.ControllerConfig != nil
}
func (this SystemConfiguration) UndefinedNode() bool {
	return !this.IsMasterNode() && !this.IsControllerNode() && !this.IsWorkerNode()
}

func (conf SystemConfiguration) Validate() error {
	return Container().Validator.Struct(conf)
}

func (conf SystemConfiguration) Ready() bool {
	return conf.Validate() == nil
}

func InitConfig() *SystemConfiguration {
	fmt.Println("Initializing config...")
	nodeId := uuid.New().String()
	var appConfig SystemConfiguration = SystemConfiguration{
		Id: nodeId,
		LocalHostConfig: LocalHostConfig{
			NetHostInterface: netutil.GetDefaultInterface().Name,
			NetHostIP:        netutil.GetDefaultIP().String(),
			NetHostname:      hostname(),
		},
		NetConfig: NetConfig{
			NetMulticastEnabled: true,
			NetUPnPPort:         1900,
		},
	}
	appConfig.ReadConfig()
	return &appConfig
}

func (config *SystemConfiguration) InitMasterNode(primary bool) *ClusterNodeConfig {
	if config.MasterNode == nil {
		config.MasterNode = &ClusterNodeConfig{
			NodeName:       "Master",
			NodeType:       Master,
			NodeAddress:    "",
			NodeMemory:     2048,
			NodeCPU:        2,
			IsJoiningNode:  !primary,
			NodeBox:        "ubuntu/xenial64",
			NodeBoxVersion: "20180831.0.0",
		}
	}
	return config.MasterNode
}

func (config *SystemConfiguration) InitWorkerNode() *ClusterNodeConfig {
	if config.WorkerNode == nil {
		config.WorkerNode = &ClusterNodeConfig{
			NodeName:       "Worker",
			NodeType:       Worker,
			NodeAddress:    "",
			NodeMemory:     2048,
			NodeCPU:        2,
			IsJoiningNode:  true,
			NodeBox:        "ubuntu/xenial64",
			NodeBoxVersion: "20180831.0.0",
		}
	}
	return config.WorkerNode
}

func (config *SystemConfiguration) InitControllerConfig() *ClusterConfig {
	if config.ControllerConfig == nil {
		config.ControllerConfig = &ClusterConfig{
			ClusterId:            "MyCluster",
			ClusterCredentials:   "MyCluster",
			ClusterPodCIDR:       "172.16.0.0/16",
			ClusterServiceDomain: "cluster.local",
			ClusterVMNet:         NAT,
			ClusterControlPlane:  "",
			ClusterAllMasters:    []ClusterNodeConfig{},
			ClusterAllWorkers:    []ClusterNodeConfig{},
			ClusterMasterApiPort: 6443,
			ClusterNetCIDR:       "192.168.99.0/24",
			ClusterToken:         "",
		}
	}
	return config.ControllerConfig
}

func (config *SystemConfiguration) ReadConfig() *Action {
	actionManager := *GetActionManager()
	action := actionManager.StartAction("Read config from " + WINKUBE_CONFIG_FILE)
	defer action.Complete()
	f, err := os.Open(WINKUBE_CONFIG_FILE)
	if err != nil {
		actionManager.LogAction(action.Id, "Could not open file: "+WINKUBE_CONFIG_FILE)
		return actionManager.CompleteWithError(action.Id, err)
	}
	defer f.Close()
	// generate config file
	var b bytes.Buffer
	_, err = b.ReadFrom(f)
	if err != nil {
		actionManager.LogAction(action.Id, "Could not read config: "+WINKUBE_CONFIG_FILE)
		return actionManager.CompleteWithError(action.Id, err)
	}
	jsonerr := json.Unmarshal(b.Bytes(), &config)
	if jsonerr != nil {
		actionManager.LogAction(action.Id, "Could not unmarshal JSON from config: "+WINKUBE_CONFIG_FILE)
		return actionManager.CompleteWithError(action.Id, err)
	}
	err = Container().Validator.Struct(config)
	if err != nil {
		action.LogActionLn("Loaded config is not valid, will trigger setup...")
		action.CompleteWithError(err)
	}
	return actionManager.LogAction(action.Id, "Config successfully read: \n\n"+fmt.Sprintf("Id: %v\nNet:%+v\nHost:%+v\nCluster:%+v\nMaster:%+v\nWorker:%+v\nNodes:%+v\n",
		config.Id,
		config.LocalHostConfig,
		config.NetConfig,
		config.ControllerConfig,
		config.MasterNode,
		config.WorkerNode))
}

func (config *SystemConfiguration) WriteConfig() *Action {
	actionManager := *GetActionManager()
	action := actionManager.StartAction("Write config to " + WINKUBE_CONFIG_FILE)
	f, err := os.Create(WINKUBE_CONFIG_FILE)
	if err != nil {
		actionManager.LogAction(action.Id, "Could not open/create file: "+WINKUBE_CONFIG_FILE)
		return actionManager.CompleteWithError(action.Id, err)
	}
	defer f.Close()
	// generate config file
	b, err := json.MarshalIndent(&config, "", "\t")
	f.Write(b)
	return actionManager.Complete(action.Id)
}

func (conf SystemConfiguration) GetHostIp() string {
	interfaces, err := net.Interfaces()
	if util2.CheckAndLogError("Failed to evaluate interfaces", err) {
		for _, iface := range interfaces {
			addresses, adrErrs := iface.Addrs()
			if util2.CheckAndLogError("Failed to evaluate addresses for interface: "+iface.Name, adrErrs) {
				for _, addr := range addresses {
					switch v := addr.(type) {
					case *net.IPNet:
						if !v.IP.IsLoopback() {
							if v.IP.To4() != nil { //Verify if IP is IPV4
								return v.IP.String()
							}
						}
					}
				}
			}
		}
	}
	return ""
}

func (conf SystemConfiguration) ClusterId() string {
	if conf.IsControllerNode() {
		return conf.ControllerConfig.ClusterId
	} else {
		return conf.ClusterLogin.ClusterId
	}
}
