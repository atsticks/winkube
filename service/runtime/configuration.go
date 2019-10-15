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

package runtime

import (
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/winkube/service/netutil"
	"github.com/winkube/service/util"
	"strconv"
	"strings"
)

type NodeType int

const (
	Undefined NodeType = iota
	WorkerNode
	MasterNode
	MonitorNode
)

func (this NodeType) String() string {
	return [...]string{"Undefined", "WorkerNode", "MasterNode", "MonitorNode"}[this]
}
func NodeType_Values() []NodeType {
	return []NodeType{
		Undefined,
		WorkerNode,
		MasterNode,
		MonitorNode,
	}
}

type NodeNetType int

const (
	NAT NodeNetType = iota
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

var appConfiguration AppConfiguration

func init() {
	appConfiguration = *CreateAppConfig("winkube.config")
}

func Get() AppConfiguration {
	return appConfiguration
}

func Set(configuration AppConfiguration) {

	appConfiguration = configuration
}

type NetConfig struct {
	NetMulticastEnabled bool
	NetUPnPPort         int
	NetLookupMasters    []string
	NetHostInterface    string `validate:"required"`
	NetHostIP           string `validate:"required"`
}

type ClusterConfig struct {
	ClusterID          string `validate:"required"`
	ClusterCredentials string
	ClusterPodCIDR     string `validate:"required"`
	ClusterVMNet       string
}

type NodeConfig struct {
	NodeNetType       NodeNetType `validate:"required"`
	NodeNetBridgeCIDR string
	NodeNetNodeIP     string
	NodeType          NodeType `validate:"required"`
	InstanceModel
}

type AppConfiguration struct {
	NetConfig     `validate:"required"`
	ClusterConfig `validate:"required"`
	NodeConfig    `validate:"required"`
}

func (conf AppConfiguration) Ready() bool {
	return false
}

func CreateAppConfig(file string) *AppConfiguration {
	appConfig := AppConfiguration{
		NetConfig{NetMulticastEnabled: true,
			NetUPnPPort:      1900,
			NetHostInterface: netutil.GetDefaultInterface().Name,
			NetHostIP:        netutil.GetDefaultIP().String(),
		},
		ClusterConfig{
			ClusterID:      "MyClusterID",
			ClusterPodCIDR: "",
			ClusterVMNet:   "",
		},
		NodeConfig{
			NodeNetType:   NAT,
			InstanceModel: *CreateDefaultInstanceModel(),
		},
	}
	props, err := util.ReadProperties("winkube.config")
	if err != nil {
		log.Info("Failed to read node properties from winkube.config")
	} else {
		applyConfig(appConfig, props)
	}
	return &appConfig
}

func applyConfig(config AppConfiguration, props util.Properties) {
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

func eval(props util.Properties, key string, defaultValue string) string {
	val := props[key]
	if val == "" {
		return defaultValue
	}
	return val
}
func evalBool(props util.Properties, key string, defaultValue bool) bool {
	val := props[key]
	if val == "" {
		return defaultValue
	}
	return strings.ToLower(strings.TrimSpace(val)) == "true"
}
func evalInt(props util.Properties, key string, defaultValue int) int {
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
func evalNodeNetType(props util.Properties, key string, defaultValue NodeNetType) NodeNetType {
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
func evalNodeType(props util.Properties, key string, defaultValue NodeType) NodeType {
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
