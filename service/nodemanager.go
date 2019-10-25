package service

import (
	"bufio"
	"fmt"
	"github.com/winkube/util"
	"gopkg.in/go-playground/validator.v9"
	"os"
	"strconv"
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
}
type VagrantConfig struct {
	ServiceNetCIDR    string
	PodNetCIDR        string      `validate:"required"`
	NodeConfig        VagrantNode `validate:"required"`
	ApiServerBindPort int         `validate:"required,gte=1"`
	ServiceDNSDomain  string      `validate:"required"`
	BridgeInterface   string      `validate:"required"`
	NetType           NodeNetType `validate:"required"`
}

func vagrantNodeValidation(sl validator.StructLevel) {

	config := sl.Current().Interface().(VagrantNode)

	if config.NodeType != MasterNode && config.NodeType != WorkerNode {
		sl.ReportError(config.NodeType, "NodeType", "IsMaster", "NodeType must be either MasterNode or WorkerNode", "")
	}
}

func createVagrantConfig(appConfig *AppConfiguration) VagrantConfig {
	config := VagrantConfig{
		ServiceNetCIDR:    appConfig.ClusterServiceCIDR,
		PodNetCIDR:        appConfig.ClusterPodCIDR,
		ApiServerBindPort: appConfig.ClusterMasterApiPort,
		ServiceDNSDomain:  appConfig.ClusterServiceDomain,
		BridgeInterface:   appConfig.NetHostInterface,
		NetType:           appConfig.NodeNetType,
		NodeConfig: VagrantNode{
			Name:       appConfig.NodeName + "-" + strconv.Itoa(appConfig.NodeIndex),
			Box:        appConfig.NodeBox,
			BoxVersion: appConfig.NodeBoxVersion,
			Ip:         appConfig.NodeNetNodeIP,
			Memory:     appConfig.NodeMemory,
			Cpu:        appConfig.NodeCPU,
			NodeType:   appConfig.NodeType,
		},
	}
	return config
}

type NodeManager interface {
	IsReady() bool
	ValidateConfig() (*VagrantConfig, error)
	Initialize(override bool) *Action
	StartNode() *Action
	StopNode() *Action
	DestroyNode() *Action
}

type nodeManager struct {
	isSetup         bool
	templateManager *util.TemplateManager
	configValidator *validator.Validate
}

func createNodeManager() *NodeManager {
	validator := validator.New()
	validator.RegisterStructValidation(vagrantNodeValidation, VagrantNode{})
	templateManager := util.CreateTemplateManager()
	templateManager.InitTemplates(map[string]string{"vagrant": "templates/vagrant/Vagrantfile"})
	var manager NodeManager = nodeManager{
		isSetup:         false,
		templateManager: templateManager,
		configValidator: validator,
	}
	return &manager
}

func (this nodeManager) IsReady() bool {
	return util.FileExists("Vagrantfile")
}

func (this nodeManager) ValidateConfig() (*VagrantConfig, error) {
	vagrantConfig := createVagrantConfig(Container().Config)
	// validate config
	return &vagrantConfig, this.configValidator.Struct(vagrantConfig)
}

func (this nodeManager) createVagrantConfig(config *AppConfiguration) (*VagrantConfig, error) {
	vagrantConfig := createVagrantConfig(config)
	// validate config
	err := this.configValidator.Struct(vagrantConfig)
	if err != nil {
		printValidationErrors(err)
	}
	return &vagrantConfig, err
}

func (this nodeManager) Initialize(override bool) *Action {
	actionManager := (Container().ActionManager)
	action := actionManager.StartAction("Initialize Node")
	if util.FileExists("Vagrantfile") {
		if override {
			_, cmdReader, err := util.RunCommand("Init Node: Stopping any running instances...", "vagrant", "destroy")
			if util.CheckAndLogError("Init Node: Stopping vagrant failed", err) {
				fmt.Println("vagrant -f destroy")
				actionManager.LogAction(action.Id, "vagrant -f destroy\n")
				scanner := bufio.NewScanner(cmdReader)
				for scanner.Scan() {
					text := (scanner.Text())
					fmt.Printf("\t%s\n", text)
					actionManager.LogAction(action.Id, text)
				}
				actionManager.LogAction(action.Id, "\n")
			} else {
				return actionManager.CompleteWithError(action.Id, err)
			}
		} else {
			actionManager.CompleteWithMessage(action.Id, "Init Node: Nothing todo: Vagrant file already configured.\n")
			return action
		}
	}
	config, err := this.createVagrantConfig(Container().Config)
	if err != nil {
		actionManager.LogAction(action.Id, "Init Node: Validation failed: "+printValidationErrors(err))
		return actionManager.CompleteWithError(action.Id, err)
	} else {
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
	}
	return action
}

func (this nodeManager) StartNode() *Action {
	actionManager := (Container().ActionManager)
	action := actionManager.StartAction("Start Node")
	go func() {
		if util.FileExists("Vagrantfile") {
			_, cmdReader, err := util.RunCommand("Start Node...", "vagrant", "up")
			if util.CheckAndLogError("Start Node: Starting vagrant failed", err) {
				fmt.Println("vagrant up")
				actionManager.LogAction(action.Id, "vagrant up\n")
				scanner := bufio.NewScanner(cmdReader)
				for scanner.Scan() {
					text := (scanner.Text())
					fmt.Printf("\t%s\n", text)
					actionManager.LogAction(action.Id, text)
				}
				actionManager.LogAction(action.Id, "\n")
			}
		} else {
			actionManager.LogAction(action.Id, "Start Node failed: not initialized.\n")
		}
		actionManager.Complete(action.Id)
	}()
	return action
}

func (this nodeManager) StopNode() *Action {
	actionManager := (Container().ActionManager)
	action := actionManager.StartAction("Stop Node")
	go func() {
		if util.FileExists("Vagrantfile") {
			_, cmdReader, err := util.RunCommand("Stopping any running instances...", "vagrant", "halt")
			if util.CheckAndLogError("Stop Node: Starting vagrant failed", err) {
				fmt.Println("vagrant halt")
				actionManager.LogAction(action.Id, "vagrant halt\n")
				scanner := bufio.NewScanner(cmdReader)
				for scanner.Scan() {
					text := (scanner.Text())
					fmt.Printf("\t%s\n", text)
					actionManager.LogAction(action.Id, text)
				}
				actionManager.LogAction(action.Id, "\n")
			}
		} else {
			actionManager.LogAction(action.Id, "Stop Node failed: not initialized.\n")
		}
		actionManager.Complete(action.Id)
	}()
	return action
}

func (this nodeManager) DestroyNode() *Action {
	actionManager := (Container().ActionManager)
	action := actionManager.StartAction("Destroy Node")
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
					actionManager.LogAction(action.Id, text)
				}
				actionManager.LogAction(action.Id, "\n")
			}
		} else {
			actionManager.LogAction(action.Id, "Destroy Node failed: not initialized.\n")
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
