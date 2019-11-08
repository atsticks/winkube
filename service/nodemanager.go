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
	"github.com/winkube/service/assert"
	"github.com/winkube/service/netutil"
	"github.com/winkube/util"
	"gopkg.in/go-playground/validator.v9"
	"os"
	"strings"
	"time"
)

type VagrantConfig struct {
	NetCIDR           string `validate:"required"`
	PodNetCIDR        string `validate:"required"`
	MasterConfig      ClusterNodeConfig
	WorkerConfig      ClusterNodeConfig
	ApiServerBindPort int       `validate:"required,gte=1"`
	ServiceDNSDomain  string    `validate:"required"`
	HostInterface     string    `validate:"required"`
	HostIp            string    `validate:"required"`
	NetType           VMNetType `validate:"required"`
	IsLocalMaster     bool
	IsLocalController bool
	PublicMaster      string
	MasterToken       string
}

//func getNodeIp(ip string, master bool) string {
//	localController := *Container().LocalController
//	if ip != "" {
//		return ip
//	}
//	if localController == nil || !localController.IsRunning() {
//		return ""
//	}
//	return localController.ReserveNodeIP(master)
//}

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
	config          *SystemConfiguration
	templateManager *util.TemplateManager
	serviceRegistry *netutil.ServiceRegistry
	running         bool
}

func createNodeManager(serviceRegistry *netutil.ServiceRegistry) *NodeManager {
	assert.AssertNotNil(serviceRegistry)
	templateManager := util.CreateTemplateManager()
	templateManager.InitTemplates(map[string]string{"vagrant": "templates/vagrant/Vagrantfile"})
	var manager NodeManager = &nodeManager{
		templateManager: templateManager,
		serviceRegistry: serviceRegistry,
	}
	return &manager
}

func (this *nodeManager) IsReady() bool {
	return this.running && util.FileExists("Vagrantfile")
}

func (this *nodeManager) ValidateConfig() error {
	return Container().Validator.Struct(config)
}

func (this *nodeManager) GetServices() []netutil.Service {
	var result []netutil.Service
	if this.config.Ready() {
		if this.config.IsControllerNode() {
			var controllerType = "remote-controller"
			if this.config.IsControllerNode() {
				controllerType = "local-controller"
			}
			result = append(result, netutil.Service{
				AdType:   WINKUBE_ADTYPE,
				Id:       this.config.Id + "-C",
				Location: "http://" + this.config.NetHostname + ":9999/cluster",
				Service:  "Controller:" + this.config.ClusterId() + ":" + controllerType,
				Version:  WINKUBE_VERSION,
				Server:   util.RuntimeInfo(),
				MaxAge:   30,
			})
		}
		if this.config.IsMasterNode() {
			result = append(result, netutil.Service{
				AdType:   WINKUBE_ADTYPE,
				Id:       this.config.Id + "-M",
				Location: "http://" + this.config.NetHostname + ":9999/master",
				Service:  "Master:" + this.config.ClusterId() + ":" + this.config.MasterNode.NodeName,
				Version:  WINKUBE_VERSION,
				Server:   util.RuntimeInfo(),
				MaxAge:   30,
			})
		}
		if this.config.IsWorkerNode() {
			result = append(result, netutil.Service{
				AdType:   WINKUBE_ADTYPE,
				Id:       this.config.Id + "-W",
				Location: "http://" + this.config.NetHostname + ":9999/worker",
				Service:  "Worker:" + this.config.ClusterId() + ":" + this.config.WorkerNode.NodeName,
				Version:  WINKUBE_VERSION,
				Server:   util.RuntimeInfo(),
				MaxAge:   30,
			})
		}
	}
	return result
}

func (this *nodeManager) DestroyNodes() *Action {
	Log().Info("Destroy nodes...")
	actionManager := *GetActionManager()
	action := actionManager.StartAction("Destroy nodes")
	defer actionManager.Complete(action.Id)
	if util.FileExists("Vagrantfile") {
		_, cmdReader, err := util.RunCommand("Stopping any running instances...", "vagrant", "destroy", "-f")
		if util.CheckAndLogError("Destroy Node: vagrant failed", err) {
			fmt.Println("vagrant destroy -f ")
			actionManager.LogAction(action.Id, "vagrant destroy -f\n")
			scanner := bufio.NewScanner(cmdReader)
			for scanner.Scan() {
				text := (scanner.Text())
				fmt.Printf("\t%s\n", text)
				actionManager.LogAction(action.Id, fmt.Sprintf("%s\n", text))
			}
			actionManager.LogAction(action.Id, "\n")
		}
		err = os.Remove("Vagrantfile")
		util.CheckAndLogError("Delete Vagrantfile", err)
	} else {
		actionManager.LogAction(action.Id, "Destroy Nodes successful: no Vagrantfile present.\n")
	}
	return action
}

func (this *nodeManager) ConfigureNodes(systemConfig SystemConfiguration, clusterConfig ClusterConfig, override bool) *Action {
	assert.AssertNotNil(systemConfig)
	assert.AssertNotNil(clusterConfig)
	actionManager := (*GetActionManager())
	action := actionManager.StartAction("Configure Nodes")
	defer this.releaseIPsOnError(action, systemConfig)
	this.config = &systemConfig
	if !this.config.IsMasterNode() && !this.config.IsWorkerNode() {
		// Only Controller will be started, no nodes!
		actionManager.CompleteWithMessage(action.Id, "Configure Nodes: no nodes to be started.\n")
		return action
	}
	config := createVagrantConfig(systemConfig, clusterConfig)
	// open file for write
	f, err := os.Create("Vagrantfile")
	if err != nil {
		action.LogAction("Could not open/create file: Vagrantfile")
		action.CompleteWithError(err)
		return action
	}
	defer f.Close()
	// generate vagrant script
	vagrantTemplate := this.templateManager.Templates["vagrant"]
	err = vagrantTemplate.Execute(f, config)
	if err != nil {
		action.LogAction("Template execution failed for Vagrantfile")
		action.CompleteWithError(err)
		return action
	}
	action.CompleteWithMessage("Init Node: Vagrantfile generated.\n")
	return action
}

func (this *nodeManager) releaseIPsOnError(action *Action, configuration SystemConfiguration) {
	if action.Error != nil {
		Log().Info("Releasing internal/public node IPs due to error: " + action.Error.Error())
		if configuration.IsMasterNode() {
			if configuration.MasterNode.NodeNetType == Bridged {
				(*Container().LocalController).ReleaseNodeIP(configuration.MasterNode.NodeAddress)
			} else {
				(*Container().LocalController).ReleaseNodeIP(configuration.MasterNode.NodeAdressInternal)
			}
		}
		if configuration.IsWorkerNode() {
			if configuration.MasterNode.NodeNetType == Bridged {
				(*Container().LocalController).ReleaseNodeIP(configuration.WorkerNode.NodeAddress)
			} else {
				(*Container().LocalController).ReleaseNodeIP(configuration.WorkerNode.NodeAdressInternal)
			}
		}
	}
}

func createVagrantConfig(systemConfiguration SystemConfiguration, clusterConfig ClusterConfig) VagrantConfig {
	config := VagrantConfig{
		NetCIDR:           clusterConfig.ClusterNetCIDR,
		PodNetCIDR:        clusterConfig.ClusterPodCIDR,
		ApiServerBindPort: clusterConfig.ClusterMasterApiPort,
		ServiceDNSDomain:  clusterConfig.ClusterServiceDomain,
		HostInterface:     systemConfiguration.NetHostInterface,
		HostIp:            systemConfiguration.GetHostIp(),
		NetType:           clusterConfig.ClusterVMNet,
		MasterConfig:      *systemConfiguration.MasterNode,
		WorkerConfig:      *systemConfiguration.WorkerNode,
		IsLocalMaster:     systemConfiguration.IsPrimaryMaster(),
		IsLocalController: systemConfiguration.IsControllerNode(),
		MasterToken:       clusterConfig.ClusterToken,
	}
	return config
}

func (this *nodeManager) StartNodes() *Action {
	assert.AssertNotNil(this.config)
	actionManager := (*GetActionManager())
	action := actionManager.StartAction("Start Nodes")
	defer actionManager.Complete(action.Id)
	if !this.config.IsWorkerNode() && !this.config.IsWorkerNode() {
		// nothing to start
		actionManager.CompleteWithMessage(action.Id, "Completed. No nodes to start.")
		return action
	}
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
	go this.publishServices()
	return action
}

func (this *nodeManager) publishServices() {
	this.running = true
	Log().Info("Starting service publish loop...")
	for this.running {
		Log().Debug("Updating service registry...")
		(*this.serviceRegistry).AddServices("NodeManager", this.GetServices())
		time.Sleep(10 * time.Second)
	}
	Log().Info("Service publish loop stopped.")
}

func (this *nodeManager) StopNodes() *Action {
	actionManager := (*GetActionManager())
	action := actionManager.StartAction("Stop Nodes...")
	this.running = false
	Log().Debug("Cleaning service registry...")
	(*this.serviceRegistry).RemoveServices("NodeManager")
	if util.FileExists("Vagrantfile") {
		_, cmdReader, err := util.RunCommand("Stopping any running instances...", "vagrant", "-f", "halt")
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
	return action
}

func (this *nodeManager) DestroyNode(name string) *Action {
	Log().Info("Destroy node: " + name + "...")
	actionManager := *GetActionManager()
	action := actionManager.StartAction("Destroy node: " + name)
	go func() {
		if util.FileExists("Vagrantfile") {
			_, cmdReader, err := util.RunCommand("Stopping node: "+name+"...", "vagrant", "-f", "destroy", name)
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

func collectNodeConfigs(clusterConfig ClusterConfig, masterNode *ClusterNodeConfig, workerNode *ClusterNodeConfig) []ClusterNodeConfig {
	var result []ClusterNodeConfig
	if masterNode != nil {
		result = append(result, *masterNode)
	}
	if workerNode != nil {
		result = append(result, *workerNode)
	}
	return result
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
