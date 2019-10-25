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
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/winkube/service/netutil"
	util2 "github.com/winkube/util"
	"gopkg.in/go-playground/validator.v9"
	"strconv"
	"strings"
)

type NodeType int

const (
	UndefinedNodeType NodeType = iota
	WorkerNode
	MasterNode
	MonitorNode
)

func (this NodeType) String() string {
	return [...]string{"UndefinedNodeType", "WorkerNode", "MasterNode", "MonitorNode"}[this]
}
func NodeType_Values() []NodeType {
	return []NodeType{
		UndefinedNodeType,
		WorkerNode,
		MasterNode,
		MonitorNode,
	}
}

type NodeNetType int

const (
	UndefinedNetType NodeNetType = iota
	NAT
	Bridged
)

func (this NodeNetType) String() string {
	return [...]string{"NAT", "Bridged"}[this]
}
func NodeNetType_Values() []NodeNetType {
	return []NodeNetType{
		NAT,
		Bridged,
	}
}

type NetConfig struct {
	NetMulticastEnabled bool
	NetUPnPPort         int
	NetLookupMasters    string
	NetHostInterface    string `validate:"required"`
	NetHostIP           string `validate:"required"`
}

type ClusterConfig struct {
	ClusterID            string `validate:"required"`
	ClusterCredentials   string
	ClusterPodCIDR       string `validate:"required"`
	ClusterServiceCIDR   string
	ClusterServiceDomain string `validate:"required"`
	ClusterVMNet         string `validate:"required"`
	ClusterMasterApiPort int    `validate:"required"`
}

type NodeConfig struct {
	NodeNetType       NodeNetType `validate:"required"`
	NodeNetBridgeCIDR string
	NodeNetNodeIP     string   `validate:"required"`
	NodeType          NodeType `validate:"required"`
	NodeIndex         int      `validate:"required"`
	NodeName          string   `validate:"required"`          // vagrant-test-1
	NodeBox           string   `validate:"required"`          // ubuntu/xenial64
	NodeBoxVersion    string   `validate:"required"`          // 20180831.0.0
	NodeMemory        int      `validate:"required,gte=1028"` // 2048
	NodeCPU           int      `validate:"required,gte=1"`    // 2
	InstanceModel
}

type AppConfiguration struct {
	NetConfig          `validate:"required"`
	ClusterConfig      `validate:"required"`
	NodeConfig         `validate:"required"`
	UseExistingCluster bool
}

func (conf AppConfiguration) Ready() bool {
	err := validator.New().Struct(Container().Config)
	if err == nil {
		return true
	}
	return false
}

func CreateAppConfig(file string, nodeIndex int) *AppConfiguration {
	var appConfig AppConfiguration = AppConfiguration{
		NetConfig{
			NetMulticastEnabled: true,
			NetUPnPPort:         1900,
			NetHostInterface:    netutil.GetDefaultInterface().Name,
			NetHostIP:           netutil.GetDefaultIP().String(),
		},
		ClusterConfig{
			ClusterID:            "MyClusterID",
			ClusterPodCIDR:       "172.16.0.0/16",
			ClusterVMNet:         "NAT",
			ClusterMasterApiPort: 6443,
			ClusterServiceDomain: "cluster.local",
		},
		NodeConfig{
			NodeNetType:       NAT,
			NodeIndex:         nodeIndex,
			NodeNetNodeIP:     "192.168.10." + strconv.Itoa(nodeIndex+1),
			NodeNetBridgeCIDR: "192.168.10.0/24",
			NodeName:          "node",
			NodeBox:           "ubuntu/xenial64",
			NodeBoxVersion:    "20180831.0.0",
			NodeMemory:        2048,
			NodeCPU:           2,
			InstanceModel:     *CreateDefaultInstanceModel(),
		},
		false,
	}
	props, err := util2.ReadProperties("winkube.config")
	if err != nil {
		log.Info("Failed to read node properties from winkube.config")
	} else {
		applyConfig(appConfig, props)
	}
	return &appConfig
}

func applyConfig(config AppConfiguration, props util2.Properties) {
	config.NetMulticastEnabled = evalBool(props, "net.multicast.enabled", config.NetMulticastEnabled)
	config.NetUPnPPort = evalInt(props, "net.upnp.port", config.NetUPnPPort)
	config.NetHostInterface = eval(props, "net.host.interface", config.NetHostInterface)
	config.NetHostIP = eval(props, "net.host.ip", config.NetHostIP)
	config.ClusterID = eval(props, "cluster.id", config.ClusterID)
	config.ClusterCredentials = eval(props, "cluster.credentials", config.ClusterCredentials)
	config.ClusterPodCIDR = eval(props, "cluster.pod-CIDR", config.ClusterPodCIDR)
	config.ClusterVMNet = eval(props, "cluster.vm.net", config.ClusterVMNet)
	config.NodeNetType = evalNodeNetType(props, "node.net.type", config.NodeNetType)
	config.NodeNetBridgeCIDR = eval(props, "node.net.bridge-CIDR", config.NodeNetBridgeCIDR)
	config.NodeNetNodeIP = eval(props, "node.net.ip", config.NodeNetNodeIP)
	config.NodeType = evalNodeType(props, "node.net.type", config.NodeType)
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
func evalNodeNetType(props util2.Properties, key string, defaultValue NodeNetType) NodeNetType {
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
