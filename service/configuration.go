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
	"github.com/sirupsen/logrus"
	"github.com/winkube/service/netutil"
	util2 "github.com/winkube/util"
	"gopkg.in/go-playground/validator.v9"
	"net"
	"os"
	"strconv"
	"strings"
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
	return [...]string{"NAT", "Bridged"}[this]
}
func NodeNetType_Values() []VMNetType {
	return []VMNetType{
		NAT,
		Bridged,
	}
}

type LocalNetConfig struct {
	NetHostInterface string `validate:"required"`
	NetHostIP        string `validate:"required"`
}

type NetConfig struct {
	NetMulticastEnabled bool
	NetUPnPPort         int `validate:"required"`
	NetLookupMaster     string
}

type ClusterLogin struct {
	ClusterId          string `validate:"required"`
	ClusterCredentials string
	Controller         *ControllerNode
}

type ClusterConfig struct {
	LocallyManaged bool
	ClusterLogin
	ClusterPodCIDR         string    `validate:"required"`
	ClusterServiceDomain   string    `validate:"required"`
	ClusterVMNet           VMNetType `validate:"required"`
	ClusterMasterIp        string    `validate:"required"`
	ClusterMasterApiPort   int
	ClusterNetCIDR         string
	ClusterInternalNetCIDR string
	ClusterToken           string `validate:"required"`
	NetConfig
}

func (this ClusterConfig) isFullConfig() bool {
	return this.ClusterPodCIDR != "" && this.ClusterServiceDomain != ""
}

type NodeConfig struct {
	NodeName   string   `validate:"required"` // node
	NodeType   NodeType `validate:"required"`
	NodeIP     string   `validate:"required"`          // host IP
	NodeMemory int      `validate:"required,gte=1028"` // 2048
	NodeCPU    int      `validate:"required,gte=1"`    // 2
}

type LocalNodeConfig struct {
	NodeConfig
	NodeBox        string `validate:"required"` // ubuntu/xenial64, centos/7
	NodeBoxVersion string `validate:"required"` // 20180831.0.0
}

func (this *NetConfig) init(config *SystemConfiguration) *NetConfig {
	this.NetLookupMaster = config.NetLookupMaster
	this.NetUPnPPort = config.NetUPnPPort
	this.NetMulticastEnabled = config.NetMulticastEnabled
	return this
}

func (this *ClusterConfig) init(config *SystemConfiguration) *ClusterConfig {
	this.ClusterId = config.ClusterLogin.ClusterId
	this.ClusterCredentials = config.ClusterLogin.ClusterCredentials
	if config.ClusterConfig != nil {
		this.ClusterNetCIDR = config.ClusterConfig.ClusterNetCIDR
		this.ClusterServiceDomain = config.ClusterConfig.ClusterServiceDomain
		this.ClusterPodCIDR = config.ClusterConfig.ClusterPodCIDR
		this.ClusterVMNet = config.ClusterConfig.ClusterVMNet
		this.ClusterCredentials = config.ClusterConfig.ClusterCredentials
	}
	return this
}

type SystemConfiguration struct {
	Id string `validate:"required", json:"id"`
	NetConfig
	LocalNetConfig
	ClusterLogin  *ClusterLogin    `validate:"required", json:"clusterLogin"`
	ClusterConfig *ClusterConfig   `json:"cluster"`
	MasterNode    *LocalNodeConfig `json:"master"`
	WorkerNode    *LocalNodeConfig `json:"worker"`
}

func (this SystemConfiguration) IsWorkerNode() bool {
	return this.WorkerNode != nil
}
func (this SystemConfiguration) IsMasterNode() bool {
	return this.MasterNode != nil
}
func (this SystemConfiguration) IsControllerNode() bool {
	return this.ClusterConfig != nil && this.ClusterConfig.LocallyManaged
}
func (this SystemConfiguration) UndefinedNode() bool {
	return !this.IsMasterNode() && !this.IsControllerNode() && !this.IsWorkerNode()
}

func (conf SystemConfiguration) Ready() bool {
	err := validator.New().Struct(Container().Config)
	if err == nil {
		return true
	}
	return false
}

func InitAppConfig() *SystemConfiguration {
	fmt.Println("Initializing config...")
	var appConfig SystemConfiguration = SystemConfiguration{
		Id: uuid.New().String(),
		ClusterLogin: &ClusterLogin{
			ClusterId:          "MyCluster",
			ClusterCredentials: "MyCluster",
		},
		ClusterConfig: &ClusterConfig{
			ClusterLogin: ClusterLogin{
				ClusterId:          "MyCluster",
				ClusterCredentials: "MyCluster",
			},
			ClusterPodCIDR:       "172.16.0.0/16",
			ClusterMasterApiPort: 6443,
			ClusterServiceDomain: "cluster.local",
			ClusterVMNet:         NAT,
			ClusterNetCIDR:       "192.168.99.0/24",
			NetConfig: NetConfig{
				NetMulticastEnabled: true,
				NetUPnPPort:         1900,
			},
		},
		NetConfig: NetConfig{
			NetMulticastEnabled: true,
			NetUPnPPort:         1900,
		},
		LocalNetConfig: LocalNetConfig{
			NetHostInterface: netutil.GetDefaultInterface().Name,
			NetHostIP:        netutil.GetDefaultIP().String(),
		},
		//Master: &LocalNodeConfig{
		//	NodeConfig: NodeConfig{
		//		NodeName:   "node",
		//		NodeType:   Master,
		//		NodeMemory: 2048,
		//		NodeCPU:    2,
		//	},
		//	NodeBox:        "ubuntu/xenial64",
		//	NodeBoxVersion: "20180831.0.0",
		//},
	}
	appConfig.readConfig()
	return &appConfig
}

func (config *SystemConfiguration) readConfig() *Action {
	actionManager := *GetActionManager()
	action := actionManager.StartAction("Read config from " + WINKUBE_CONFIG_FILE)
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
	err = validator.New().Struct(config)
	if err != nil {
		actionManager.LogAction(action.Id, "Loaded config is not valid, will trigger setup, error: "+err.Error())
	}
	return actionManager.CompleteWithMessage(action.Id, "Config successfully read: \n\n"+fmt.Sprintf("Id: %v\nNet:%+v\nHost:%+v\nCluster:%+v\nMaster:%+v\nWorker:%+v\nNodes:%+v\n",
		config.Id,
		config.NetConfig,
		config.LocalNetConfig,
		config.ClusterConfig,
		config.MasterNode,
		config.WorkerNode))
}

func (config *SystemConfiguration) writeConfig() *Action {
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

func eval(props util2.Properties, key string, defaultValue string) string {
	val := props[key]
	if val == "" {
		return defaultValue
	}
	return val
}
func evalBool(props util2.Properties, key string, defaultValue bool) bool {
	val := props[key]
	if val == "" {
		return defaultValue
	}
	return strings.ToLower(strings.TrimSpace(val)) == "true"
}
func evalInt(props util2.Properties, key string, defaultValue int) int {
	val := props[key]
	if val == "" {
		return defaultValue
	}
	r, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil {
		logrus.Info("Invalid config entry for " + key + " ; " + val)
		return defaultValue
	}
	return r
}
func evalNodeNetType(props util2.Properties, key string, defaultValue VMNetType) VMNetType {
	val := props[key]
	if val == "" {
		return defaultValue
	}
	for _, nt := range NodeNetType_Values() {
		if nt.String() == val {
			return nt
		}
	}
	return defaultValue
}
func evalNodeType(props util2.Properties, key string, defaultValue NodeType) NodeType {
	val := props[key]
	if val == "" {
		return defaultValue
	}
	for _, nt := range NodeType_Values() {
		if nt.String() == val {
			return nt
		}
	}
	return defaultValue
}
