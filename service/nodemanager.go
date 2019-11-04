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
	"bufio"
	"fmt"
	"github.com/winkube/service/netutil"
	"github.com/winkube/util"
	"gopkg.in/go-playground/validator.v9"
	"os"
	"strings"
)

type VagrantNode struct {
	Name       string   `validate:"required"`
	NodeType   NodeType `validate:"required"`
	Box        string   `validate:"required"`
	BoxVersion string   `validate:"required"`
	Ip         string   `validate:"required,ip"`
	Memory     int      `validate:"required,gte=1024"`
	Cpu        int      `validate:"required,gte=1"`
	NetType    string   `validate:"required"`
	Joinining  bool
}
type VagrantConfig struct {
	NetCIDR           string `validate:"required"`
	PodNetCIDR        string `validate:"required"`
	NodeConfigs       []VagrantNode
	ApiServerBindPort int       `validate:"required,gte=1"`
	ServiceDNSDomain  string    `validate:"required"`
	HostInterface     string    `validate:"required"`
	HostIp            string    `validate:"required"`
	NetType           VMNetType `validate:"required"`
	LocalMaster       string
	PublicMaster      string
	MasterToken       string
}

func createNodeConfigs(clusterConfig ClusterConfig, masterNode *ClusterNodeConfig, workerNode *ClusterNodeConfig) []VagrantNode {
	var result []VagrantNode
	if masterNode != nil {
		node := VagrantNode{
			Name:       clusterConfig.ClusterId + "-" + masterNode.NodeType.String() + "-" + hostname(),
			Box:        masterNode.NodeBox,
			BoxVersion: masterNode.NodeBoxVersion,
			Ip:         getNodeIp(masterNode.NodeIP, true),
			Memory:     masterNode.NodeMemory,
			Cpu:        masterNode.NodeCPU,
			NodeType:   masterNode.NodeType,
			NetType:    clusterConfig.ClusterVMNet.String(),
			Joinining:  masterNode.IsJoiningNode,
		}
		result = append(result, node)
	}
	if workerNode != nil {
		node := VagrantNode{
			Name:       clusterConfig.ClusterId + "-" + workerNode.NodeType.String() + "-" + hostname(),
			Box:        workerNode.NodeBox,
			BoxVersion: workerNode.NodeBoxVersion,
			Ip:         getNodeIp(workerNode.NodeIP, false),
			Memory:     workerNode.NodeMemory,
			Cpu:        workerNode.NodeCPU,
			NodeType:   workerNode.NodeType,
			NetType:    clusterConfig.ClusterVMNet.String(),
			Joinining:  true,
		}
		result = append(result, node)
	}
	return result
}

func getNodeIp(ip string, master bool) string {
	localController := *Container().LocalController
	if ip != "" {
		return ip
	}
	if localController == nil || !localController.IsRunning() {
		return ""
	}
	return localController.ReserveNodeIP(master)
}

type NodeManager interface {
	IsReady() bool
	ValidateConfig() error
	ConfigureNodes(systemConfig SystemConfiguration, clusterConfig ClusterConfig, override bool) *Action
	StartNodes() *Action
	StopNodes() *Action
	DestroyNodes() *Action
	DestroyNode(name string) *Action
	GetServices() []netutil.Service
}

type nodeManager struct {
	isSetup         bool
	templateManager *util.TemplateManager
}

func createNodeManager() *NodeManager {

	templateManager := util.CreateTemplateManager()
	templateManager.InitTemplates(map[string]string{"vagrant": "templates/vagrant/Vagrantfile"})
	var manager NodeManager = nodeManager{
		isSetup:         false,
		templateManager: templateManager,
	}
	return &manager
}

func (this nodeManager) IsReady() bool {
	return util.FileExists("Vagrantfile")
}

func (this nodeManager) ValidateConfig() error {
	return Container().Validator.Struct(Container().Config)
}

func (this nodeManager) GetServices() []netutil.Service {
	config := Container().Config
	var result []netutil.Service
	if config.Ready() {
		if config.IsControllerNode() {
			result = append(result, netutil.Service{
				AdType:   WINKUBE_ADTYPE,
				Id:       config.Id,
				Location: config.NetHostIP + ":9999",
				Service:  "Controller:" + config.ClusterId(),
				Version:  WINKUBE_VERSION,
				Server:   util.RuntimeInfo(),
				MaxAge:   5,
			})
		}
		if config.IsMasterNode() {
			result = append(result, netutil.Service{
				AdType:   WINKUBE_ADTYPE,
				Id:       config.Id,
				Location: config.MasterNode.NodeIP + ":9999",
				Service:  "Master:" + config.ClusterId(),
				Version:  WINKUBE_VERSION,
				Server:   util.RuntimeInfo(),
				MaxAge:   5,
			})
		}
		if config.IsWorkerNode() {
			result = append(result, netutil.Service{
				AdType:   WINKUBE_ADTYPE,
				Id:       config.Id,
				Location: config.WorkerNode.NodeIP + ":9999",
				Service:  "Master:" + config.ClusterId(),
				Version:  WINKUBE_VERSION,
				Server:   util.RuntimeInfo(),
				MaxAge:   5,
			})
		}
	}
	return result
}

func (this nodeManager) ConfigureNodes(systemConfig SystemConfiguration, clusterConfig ClusterConfig, override bool) *Action {
	actionManager := (*GetActionManager())
	action := actionManager.StartAction("Configure Nodes")

	if !Container().Config.IsMasterNode() && !Container().Config.IsWorkerNode() {
		// Only Controller will be started, no nodes!
		actionManager.CompleteWithMessage(action.Id, "Configure Nodes: no nodes to be started.\n")
		return action
	}
	config := createVagrantConfig(systemConfig, clusterConfig)
	// open file for write
	f, err := os.Create("Vagrantfile")
	if err != nil {
		actionManager.LogAction(action.Id, "Could not open/create file: Vagrantfile")
		return actionManager.CompleteWithError(action.Id, err)
	}
	defer f.Close()
	// generate vagrant script
	vagrantTemplate := this.templateManager.Templates["vagrant"]
	err = vagrantTemplate.Execute(f, config)
	if err != nil {
		actionManager.LogAction(action.Id, "Template execution failed for Vagrantfile")
		return actionManager.CompleteWithError(action.Id, err)
	}
	actionManager.CompleteWithMessage(action.Id, "Init Node: Vagrantfile generated.\n")
	return action
}

func createVagrantConfig(appConfig SystemConfiguration, clusterConfig ClusterConfig) VagrantConfig {
	config := VagrantConfig{
		NetCIDR:           clusterConfig.ClusterNetCIDR,
		PodNetCIDR:        clusterConfig.ClusterPodCIDR,
		ApiServerBindPort: clusterConfig.ClusterMasterApiPort,
		ServiceDNSDomain:  clusterConfig.ClusterServiceDomain,
		HostInterface:     appConfig.NetHostInterface,
		HostIp:            appConfig.GetHostIp(),
		NetType:           clusterConfig.ClusterVMNet,
		NodeConfigs:       createNodeConfigs(clusterConfig, appConfig.MasterNode, appConfig.WorkerNode),
	}
	if appConfig.MasterNode != nil {
		config.LocalMaster = appConfig.MasterNode.NodeIP
		if config.PublicMaster == "" {
			config.PublicMaster = config.LocalMaster
		}
	}
	return config
}

func (this nodeManager) StartNodes() *Action {
	actionManager := (*GetActionManager())
	action := actionManager.StartAction("Start Nodes")
	if !Container().Config.IsWorkerNode() && !Container().Config.IsWorkerNode() {
		// nothing to start
		actionManager.CompleteWithMessage(action.Id, "Completed. No nodes to start.")
		return action
	}

	go func() {
		if util.FileExists("Vagrantfile") {
			_, cmdReader, err := util.RunCommand("start Nodes...", "vagrant", "up")
			if util.CheckAndLogError("start Nodes: Starting vagrant failed", err) {
				fmt.Println("vagrant up")
				actionManager.LogAction(action.Id, "vagrant up\n")
				scanner := bufio.NewScanner(cmdReader)
				for scanner.Scan() {
					text := (scanner.Text())
					fmt.Printf("\t%s\n", text)
					actionManager.LogAction(action.Id, fmt.Sprintf("%s\n", text))
				}
				actionManager.LogAction(action.Id, "\n")
			}
		} else {
			actionManager.LogAction(action.Id, "start Nodes failed: not configured.\n")
		}
		actionManager.Complete(action.Id)
	}()
	return action
}

func (this nodeManager) StopNodes() *Action {
	actionManager := (*GetActionManager())
	action := actionManager.StartAction("Stop Nodes...")
	go func() {
		if util.FileExists("Vagrantfile") {
			_, cmdReader, err := util.RunCommand("Stopping any running instances...", "vagrant", "halt")
			if util.CheckAndLogError("Stop Nodes: Starting vagrant failed", err) {
				fmt.Println("vagrant halt")
				actionManager.LogAction(action.Id, "vagrant halt\n")
				scanner := bufio.NewScanner(cmdReader)
				for scanner.Scan() {
					text := (scanner.Text())
					fmt.Printf("\t%s\n", text)
					actionManager.LogAction(action.Id, fmt.Sprintf("%s\n", text))
				}
				actionManager.LogAction(action.Id, "\n")
			}
		} else {
			actionManager.LogAction(action.Id, "Stop Nodes failed: not configured.\n")
		}
		actionManager.Complete(action.Id)
	}()
	return action
}

func (this nodeManager) DestroyNodes() *Action {
	Log().Info("Destroy nodes...")
	actionManager := *GetActionManager()
	action := actionManager.StartAction("Destroy nodes")
	go func() {
		if util.FileExists("Vagrantfile") {
			_, cmdReader, err := util.RunCommand("Stopping any running instances...", "vagrant", "destroy")
			if util.CheckAndLogError("Destroy Node: Starting vagrant failed", err) {
				fmt.Println("vagrant -f destroy")
				actionManager.LogAction(action.Id, "vagrant -f destroy\n")
				scanner := bufio.NewScanner(cmdReader)
				for scanner.Scan() {
					text := (scanner.Text())
					fmt.Printf("\t%s\n", text)
					actionManager.LogAction(action.Id, fmt.Sprintf("%s\n", text))
				}
				actionManager.LogAction(action.Id, "\n")
			}
		} else {
			actionManager.LogAction(action.Id, "Destroy Nodes failed: not initialized.\n")
		}
		actionManager.Complete(action.Id)
	}()
	return action
}

func (this nodeManager) DestroyNode(name string) *Action {
	Log().Info("Destroy node: " + name + "...")
	actionManager := *GetActionManager()
	action := actionManager.StartAction("Destroy node: " + name)
	go func() {
		if util.FileExists("Vagrantfile") {
			_, cmdReader, err := util.RunCommand("Stopping node: "+name+"...", "vagrant", "-f destroy "+name)
			if util.CheckAndLogError("Destroy Node: vagrant error occurred.", err) {
				fmt.Println("vagrant -f destroy " + name)
				actionManager.LogAction(action.Id, "vagrant -f destroy "+name+"\n")
				scanner := bufio.NewScanner(cmdReader)
				for scanner.Scan() {
					text := (scanner.Text())
					fmt.Printf("\t%s\n", text)
					actionManager.LogAction(action.Id, fmt.Sprintf("%s\n", text))
				}
				actionManager.LogAction(action.Id, "\n")
			}
		} else {
			actionManager.LogAction(action.Id, "Destroy Node: "+name+" failed: not initialized.\n")
		}
		actionManager.Complete(action.Id)
	}()
	return action
}

func printValidationErrors(err error) string {
	b := strings.Builder{}
	for _, err := range err.(validator.ValidationErrors) {
		b.WriteString(err.Namespace())
		b.WriteString("\n")
		b.WriteString(err.Field())
		b.WriteString("\n")
		b.WriteString(err.StructNamespace()) //
		b.WriteString("\n")                  // can differ when a custom TagNameFunc is registered or
		b.WriteString(err.StructField())
		b.WriteString("\n") // by passing alt name to ReportError like below
		b.WriteString(err.Tag())
		b.WriteString("\n")
		b.WriteString(err.ActualTag())
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("%b", err.Kind()))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("%b", err.Type()))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("%b", err.Value()))
		b.WriteString("\n")
		b.WriteString(err.Param())
		b.WriteString("\n")
	}
	return b.String()
}
