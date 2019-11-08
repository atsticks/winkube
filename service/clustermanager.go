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
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/winkube/service/netutil"
	"github.com/winkube/util"
	"github.com/winkube/webapp"
	"golang.org/x/text/language"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Endpoint int

const (
	ControllerEndpoint Endpoint = iota + 1
	UIEndpoint
	MasterEndpoint
)

type Node struct {
	Id        string    `json:"id"`
	ClusterId string    `json:"ClusterId"`
	NodeType  NodeType  `json:"nodeType"`
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	Timestamp time.Time `json:"timestamp"`
	Endpoint  string    `json:"endpoint"`
}

type Cluster struct {
	ClusterConfig *ClusterConfig  `json:"config"`
	Controller    *Node           `json:"controllerConnection"`
	Masters       map[string]Node `json:"masters"`
	Workers       map[string]Node `json:"workers"`
}

func (this Cluster) removeNode(node *Node) {
	delete(this.Workers, node.Id)
	delete(this.Masters, node.Id)
}

type ControllerDelegate interface {
	Start() error
	Stop() error
	GetClusterId() string
	GetClusterConfig() ClusterConfig
	Exec(command string) string
	ReserveNodeIP(master bool) string
	ReleaseNodeIP(string)
}

type LocalController interface {
	IsRunning() bool
	Start(config *SystemConfiguration) error
	Stop() error
	GetClusterId() string
	GetClusterConfig() ClusterConfig
	GetState() string
	ReserveNodeIP(master bool) string
	ReleaseNodeIP(string)
	DrainNode(node Node)
	CordonNode(node Node)

	GetKnownClusters() []Cluster
	GetClusterById(clusterId string) *Cluster
	UpdateCluster(clusterId string) *Cluster
}

func CreateLocalController(serviceRegistry *netutil.ServiceRegistry) *LocalController {
	var cm = localController{
		knownClusters:   make(map[string]*Cluster),
		serviceRegistry: serviceRegistry,
	}
	var CM LocalController = &cm
	return &CM
}

func createLocalControllerNode(clusterId string, nodeId string) *Node {
	cn := Node{
		Id:        nodeId,
		ClusterId: clusterId,
		NodeType:  Controller,
		Host:      hostname(),
		Timestamp: time.Now(),
		Endpoint:  "http://" + hostname() + ":9999/cluster",
	}
	return &cn
}

// This creates the cluster API application serving cluster data to other Nodes.
// This application is active only, if this node is configured as a cluster
// controllerConnection.
func createClusterManagerWebApp(controller *localControllerDelegate) *webapp.WebApplication {
	webapp := webapp.CreateWebApp("cluster", "", language.English)
	webapp.GetAction("/cluster/id", controller.actionClusterId)
	webapp.GetAction("/cluster/known", actionKnownIds)
	webapp.GetAction("/cluster", controller.actionServeClusterConfig)
	webapp.GetAction("/cluster/ClusterState", actionClusterState)
	webapp.GetAction("/cluster/nodeip", controller.actionReserveNodeIP)
	webapp.DeleteAction("/cluster/nodeip", controller.actionReleaseNodeIP)
	webapp.PostAction("/cluster/node", controller.actionNodeStarted)
	webapp.DeleteAction("/cluster/node", controller.actionNodeStopped)
	webapp.GetAction("/cluster/masters", controller.actionGetMasters)
	webapp.GetAction("/cluster/workers", controller.actionGetWorkers)
	webapp.GetAction("/master", actionMasterState)
	webapp.GetAction("/worker", actionWorkerState)
	webapp.GetAction("/master/exec", controller.actionMasterExecCommand)
	webapp.GetAction("/worker/exec", controller.actionWorkerExecCommand)
	return webapp
}

// The cluster manager is the proxy management component which connects this machine with the overall
// controllerConnection. If the controllerConnection is locally, the this component also manages the controllerConnection
// api which is used by other Nodes.
type localController struct {
	serviceRegistry    *netutil.ServiceRegistry `validate:"required"`
	controllerDelegate *ControllerDelegate      `validate:"required"`
	clusterId          string                   `validate:"required"`
	knownClusters      map[string]*Cluster
}

func (c *localController) Start(config *SystemConfiguration) error {
	Log().Info("Starting local controller...")
	var err error
	if config.IsControllerNode() {
		err = c.startLocal(config, config.Id)
	} else {
		err = c.startRemote(*config.ClusterLogin)
	}
	if !util.CheckAndLogError("Failed to start local controller.", err) {
		return err
	}
	if config.IsMasterNode() {
		c.configureMaster(config)
	}
	if config.IsWorkerNode() {
		c.configureWorker(config)
	}
	err = Container().Validator.Struct(c)
	if !util.CheckAndLogError("Failed to configure local Nodes.", err) {
		return err
	}
	var l netutil.ServiceListener = *c
	(*c.serviceRegistry).Listen(&l)
	return nil
}

func (c localController) ServiceReceived(service netutil.Service) {
	if strings.Index(service.AdType, "winkube-org:") < 0 {
		return
	}
	node := nodeFromService(service)
	cluster := c.GetOrCreateClusterById(node.ClusterId)
	switch node.NodeType {
	case Master:
		cluster.Masters[node.Id] = *node
	case Worker:
		cluster.Workers[node.Id] = *node
	case Controller:
		cluster.Controller = node
	}
}

func (this *localController) configureMaster(configuration *SystemConfiguration) {
	configuration.MasterNode.NodeNetType = configuration.ControllerConfig.ClusterVMNet
	if configuration.ControllerConfig.ClusterVMNet == NAT {
		configuration.MasterNode.NodeAddress = (*this.controllerDelegate).ReserveNodeIP(true)
		configuration.MasterNode.NodeAdressInternal = configuration.MasterNode.NodeAddress
	} else if configuration.ControllerConfig.ClusterVMNet == Bridged {
		configuration.MasterNode.NodeAddress = configuration.LocalHostConfig.NetHostname
		configuration.MasterNode.NodeAdressInternal = (*this.controllerDelegate).ReserveNodeIP(true)
	} else {
		panic("Unsupported net type found")
	}
}
func (this *localController) configureWorker(configuration *SystemConfiguration) {
	configuration.WorkerNode.NodeNetType = configuration.ControllerConfig.ClusterVMNet
	if configuration.ControllerConfig.ClusterVMNet == Bridged {
		configuration.WorkerNode.NodeAddress = (*this.controllerDelegate).ReserveNodeIP(false)
		configuration.WorkerNode.NodeAdressInternal = configuration.WorkerNode.NodeAddress
	} else if configuration.ControllerConfig.ClusterVMNet == NAT {
		configuration.WorkerNode.NodeAddress = configuration.LocalHostConfig.NetHostname
		configuration.WorkerNode.NodeAdressInternal = (*this.controllerDelegate).ReserveNodeIP(true)
	} else {
		panic("Unsupported net type found")
	}
}

func (c *localController) ReserveNodeIP(master bool) string {
	c.ensureRunning()
	return (*c.controllerDelegate).ReserveNodeIP(master)
}

func (c *localController) ReleaseNodeIP(ip string) {
	c.ensureRunning()
	(*c.controllerDelegate).ReleaseNodeIP(ip)
}

// Checks if this component is running, panics otherwise
func (c *localController) ensureRunning() {
	if !c.IsRunning() {
		panic("Local controller is not running")
	}
}

func (c *localController) DrainNode(node Node) {
	cluster := c.GetClusterById(node.ClusterId)
	if cluster != nil {
		controller := cluster.Controller
		if controller != nil {
			c.execRemote(controller, "kubectl drain node "+node.Name)
		}
	}
}

func (c *localController) CordonNode(node Node) {
	cluster := c.GetClusterById(node.ClusterId)
	if cluster != nil {
		controller := cluster.Controller
		if controller != nil {
			c.execRemote(controller, "kubectl cordon node "+node.Name)
		}
	}
}

func (c *localController) IsRunning() bool {
	return c.controllerDelegate != nil
}

func (c *localController) GetState() string {
	return (*c.controllerDelegate).Exec("kubectl get Nodes")
}

func (this *localController) startLocal(config *SystemConfiguration, nodedId string) error {
	Log().Info("Starting local cluster controller for cluster: " + config.ControllerConfig.ClusterId)
	this.clusterId = config.ControllerConfig.ClusterId
	clusterState := this.knownClusters[config.ControllerConfig.ClusterId]
	if clusterState == nil {
		clusterState = &Cluster{
			ClusterConfig: config.ControllerConfig,
			Controller:    createLocalControllerNode(config.ControllerConfig.ClusterId, nodedId),
			Masters:       make(map[string]Node),
			Workers:       make(map[string]Node),
		}
		this.knownClusters[config.ControllerConfig.ClusterId] = clusterState
	} else {
		if clusterState.Controller.Host != hostname() {
			panic(fmt.Sprint("Cluster is remotedly managed. Cannot start a local controllerConnection for &v", config.ClusterId))
		}
	}
	clController := localControllerDelegate{
		clusterState:   clusterState,
		clusterNetCIDR: netutil.CreateCIDR(config.ControllerConfig.ClusterNetCIDR),
	}
	var cctl ControllerDelegate = &clController
	this.controllerDelegate = &cctl
	err := Container().Validator.Struct(this)
	if !util.CheckAndLogError("Failed to start local controllerConnection.", err) {
		panic(err)
	}
	return clController.Start()
}

func (this *localController) startRemote(clusterConnection ClusterControllerConnection) error {
	Log().Info("Connecting to remote cluster: " + clusterConnection.ClusterId + "...")
	clusterConfig, err := loadRemoteConfig(clusterConnection)
	if !util.CheckAndLogError("Failed to start local controllerConnection.", err) {
		return err
	}
	clController := remoteControllerDelegate{
		controllerConnection: clusterConnection,
		config:               clusterConfig,
	}
	var cctl ControllerDelegate = clController
	this.controllerDelegate = &cctl
	err = Container().Validator.Struct(this)
	if !util.CheckAndLogError("Failed to start cluster manager.", err) {
		panic(err)
	}
	return clController.Start()
}

func (this *localController) Stop() error {
	if this.controllerDelegate != nil {
		(*this.controllerDelegate).Stop()
		this.controllerDelegate = nil
	}
	return nil
}

func (this *localController) GetClusterById(clusterId string) *Cluster {
	return this.knownClusters[clusterId]
}

func (this *localController) UpdateCluster(clusterId string) *Cluster {
	state := this.knownClusters[clusterId]
	if state != nil {
		if this.clusterId != state.ClusterConfig.ClusterId {
			// TODO Update Cluster from remote controllerConnection
		}
	}
	return state
}

func (this *localController) GetClusterId() string {
	return this.clusterId
}
func (this *localController) GetClusterConfig() ClusterConfig {
	if this.IsRunning() {
		return (*this.controllerDelegate).GetClusterConfig()
	}
	panic("Not running")
}

func (this *localController) GetCluster() *Cluster {
	return this.GetClusterById(this.GetClusterId())
}

func (this *localController) GetKnownClusters() []Cluster {
	clusters := []Cluster{}
	for _, v := range this.knownClusters {
		clusters = append(clusters, *v)
	}
	return clusters
}

func (this *localController) GetOrCreateClusterById(clusterId string) *Cluster {
	cluster := this.knownClusters[clusterId]
	if cluster == nil {
		cluster = &Cluster{
			Masters: make(map[string]Node),
			Workers: make(map[string]Node),
		}
	}
	return cluster
}

func (this *localController) UpdateAndGetClusterById(clusterName string) *Cluster {
	// TODO perform update
	return this.GetClusterById(clusterName)
}

// Execute the given command on the (controller) node given.
func (c *localController) execRemote(node *Node, command string) {
	panic("execRemote not implemented!")
}

// A remote ClusterControlPane is an passive management component that delegates cluster management to the
// current active cluster controllerConnection, which resideds on another host. It caches and regularly updates
// current cloud configuration from its master controllerConnection.
type remoteControllerDelegate struct {
	controllerConnection ClusterControllerConnection `validate:"required"`
	config               *ClusterConfig              // will be loaded from the controllerConnection...
}

func (r remoteControllerDelegate) Start() error {
	config, err := loadRemoteConfig(r.controllerConnection)
	if err == nil {
		r.config = config
	}
	return err
}

func (r remoteControllerDelegate) Exec(command string) string {
	data, err := performGet("http://" + r.controllerConnection.ControllerHost + ":9999/cluster/exec&cmd=" + command)
	if err != nil {
		Log().Error("Exec", err)
		return ""
	}
	return string(data)
}

func (r remoteControllerDelegate) Stop() error {
	// nothing to do
	return nil
}

func (r remoteControllerDelegate) GetClusterId() string {
	if r.config != nil {
		return r.config.ClusterId
	}
	return ""
}

func (r remoteControllerDelegate) getConfig() ClusterConfig {
	if r.config == nil {
		// TODO load config from controllerConnection
	}
	return *r.config
}

func (r remoteControllerDelegate) GetClusterConfig() ClusterConfig {
	return r.getConfig()
}

func (r remoteControllerDelegate) GetState() string {
	return r.masterRemoteExec("kubectl get Nodes")
}

func (r remoteControllerDelegate) GetMasters() []Node {
	// Call controllerConnection to get master list
	data, err := performGet("http://" + r.controllerConnection.ControllerHost + ":9999/cluster/masters")
	if err != nil {
		Log().Error("GetMasters", err)
		return []Node{}
	}
	var nodes []Node
	err = json.Unmarshal(data, nodes)
	if err != nil {
		Log().Error("GetMasters", err)
		return []Node{}
	}
	return nodes
}

func (r remoteControllerDelegate) GetWorkers() []Node {
	// Call controllerConnection to get worker list
	data, err := performGet("http://" + r.controllerConnection.ControllerHost + ":9999/cluster/workers")
	if err != nil {
		Log().Error("GetWorkers", err)
		return []Node{}
	}
	var nodes []Node
	err = json.Unmarshal(data, nodes)
	if err != nil {
		Log().Error("GetWorkers", err)
		return []Node{}
	}
	return nodes
}

func (r remoteControllerDelegate) ReserveNodeIP(master bool) string {
	// Call controllerConnection to get ip
	data, err := performGet("http://" + r.controllerConnection.ControllerHost + ":9999/cluster/nodeip")
	if err != nil {
		Log().Error("ReserveNodeIP", err)
		return ""
	}
	return string(data)
}

func (r remoteControllerDelegate) ReleaseNodeIP(ip string) {
	// Call controllerConnection to release ip
	_, err := performDelete("http://" + r.controllerConnection.ControllerHost + ":9999/cluster/nodeip?ip=" + ip)
	if err != nil {
		Log().Error("ReleaseNodeIP", err)
	}
}

func (r remoteControllerDelegate) DrainNode(node Node) {
	r.masterRemoteExec("kubectl drain " + node.Name)
}

func (r remoteControllerDelegate) CordonNode(node Node) {
	r.masterRemoteExec("kubectl cordon " + node.Name)
}

func (r remoteControllerDelegate) masterRemoteExec(command string) string {
	// Call controllerConnection to get internal ip
	data, err := performGet("http://" + r.controllerConnection.ControllerHost + ":9999/cluster/exec?cmd=" + command)
	if err != nil {
		Log().Error("masterRemoteExec", err)
		return ""
	}
	return string(data)
}

// A ClusterControlPane is an active management component that manages a cluster. It trackes the
// Nodes (masters and workers) in the knownClusters, the IP addresses used for bridge Nodes (VMNetCIDR) as
// well as for internal NAT addressing (internalNetCIDR) and finally the credentials for joining
// the cluster.
type localControllerDelegate struct {
	clusterState   *Cluster `validate:"required"`
	clusterNetCIDR *netutil.CIDR
	server         *http.Server
}

func (r *localControllerDelegate) GetState() string {
	return r.Exec("kubectl get Nodes")
}

func (c *localControllerDelegate) Start() error {
	// initialize CIDR managers
	c.clusterNetCIDR = netutil.CreateCIDR(c.clusterState.ClusterConfig.ClusterNetCIDR)
	// start the cloud server
	Log().Info("Initializing controllerConnection api...")
	router := mux.NewRouter()
	clusterApiApp := createClusterManagerWebApp(c)
	router.PathPrefix("/").HandlerFunc(clusterApiApp.HandleRequest)
	c.server = &http.Server{Addr: "0.0.0.0:9999", Handler: router}
	go c.listenHttp()
	return nil
}

func (c *localControllerDelegate) listenHttp() {
	c.server.ListenAndServe()
}

func (c *localControllerDelegate) Stop() error {
	if c.server != nil {
		return c.server.Close()
	}
	return nil
}

func (c *localControllerDelegate) GetClusterId() string {
	return c.clusterState.ClusterConfig.ClusterId
}

func (c *localControllerDelegate) GetClusterConfig() ClusterConfig {
	return *c.clusterState.ClusterConfig
}

func (c *localControllerDelegate) GetMasters() []Node {
	result := []Node{}
	for _, v := range c.clusterState.Masters {
		result = append(result, v)
	}
	return result
}

func (c *localControllerDelegate) GetWorkers() []Node {
	result := []Node{}
	for _, v := range c.clusterState.Workers {
		result = append(result, v)
	}
	return result
}

func (c *localControllerDelegate) ReserveNodeIP(master bool) string {
	if c.clusterNetCIDR == nil {
		return ""
	}
	return (*c.clusterNetCIDR).GetFreeIp()
}

func (c *localControllerDelegate) ReleaseNodeIP(ip string) {
	if c.clusterNetCIDR == nil {
		return
	}
	(*c.clusterNetCIDR).MarkIpUnused(ip)
}

func (c *localControllerDelegate) DrainNode(node Node) {
	c.Exec("kubectl drain " + node.Name)
}

func (c *localControllerDelegate) CordonNode(node Node) {
	c.Exec("kubectl cordon " + node.Name)
}

func (c *localControllerDelegate) Exec(command string) string {
	// TODO implement remote master exec...
	fmt.Println("TODO implement remote master exec: " + command)
	return "Not implemented: " + command
}

// Evaluates the cluster id as the right part of the WinKube service identifier:
// e.g. 'master:myCluster01' results in 'myCluster01'
func getClusterId(service netutil.Service) string {
	// format: service:ClusterId:version
	splits := strings.Split(service.Service, ":")
	return splits[1]
}

// Evaluates the service id as the left part of the WinKube service identifier:
// e.g. 'master:myCluster01' results in 'master'
func getServiceIdentifier(service netutil.Service) string {
	// format: service:ClusterId:version
	splits := strings.Split(service.Service, ":")
	return splits[0]
}

// Evaluates the service id as the left part of the WinKube service identifier:
// e.g. 'master:myCluster01' results in 'master'
func getNodeName(service netutil.Service) string {
	// format: service:ClusterId:version
	splits := strings.Split(service.Service, ":")
	return splits[2]
}

// Evaluates the service id as the left part of the WinKube service identifier:
// e.g. 'master:myCluster01' results in 'master'
func getServiceVersion(service netutil.Service) string {
	// format: service:ClusterId:version
	splits := strings.Split(service.Service, ":")
	return splits[3]
}

// Evaluates the nodetype based on the current service identifier
func getNodeType(service netutil.Service) NodeType {
	switch strings.ToLower(getServiceIdentifier(service)) {
	case "master":
		return Master
	case "worker":
		return Worker
	case "controller":
		return Controller
	default:
		return UndefinedNode
	}
}

//func (this Cluster) registerService(service netutil.Service) {
//	//// check, if already registered as node
//	Log().Debug("Checking if instance is a node...", service)
//
//}

func (this Cluster) getNodeByService(service *netutil.Service) *Node {
	return this.getNode(service.Id)
}
func (this Cluster) getNode(serviceId string) *Node {
	for _, item := range this.Masters {
		if serviceId == item.Id {
			return &item
		}
	}
	for _, item := range this.Workers {
		if serviceId == item.Id {
			return &item
		}
	}
	return nil
}

func hostname() string {
	var hn string
	hn, _ = os.Hostname()
	return hn
}

// Web application actions...

func (this localControllerDelegate) actionClusterId(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	writer.Write([]byte(this.clusterState.ClusterConfig.ClusterId))
	writer.Header().Set("Content-Type", "text/plain")
	writer.WriteHeader(http.StatusOK)
	return nil
}

func (this localControllerDelegate) actionServeClusterConfig(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	data, err := json.MarshalIndent(this.clusterState.ClusterConfig, "", "  ")
	if err != nil {
		writer.Write([]byte("Failed to serialize config to JSON: " + err.Error()))
		writer.WriteHeader(http.StatusInternalServerError)
	} else {
		writer.Write(data)
		writer.Header().Set("Content-Type", "application/json")
	}
	return nil
}

func (this localControllerDelegate) actionReserveNodeIP(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	master := util.ParseBool(context.GetQueryParameter("master"))
	ip := this.ReserveNodeIP(master)
	if ip == "" {
		writer.WriteHeader(http.StatusNotFound)
		return nil
	}
	writer.Write([]byte(ip))
	writer.Header().Set("Content-Type", "text/plain")
	return nil
}

func (this localControllerDelegate) actionReleaseNodeIP(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	address := context.GetQueryParameter("address")
	if address == "" {
		writer.Write([]byte("Parameter 'address' missing."))
		writer.WriteHeader(http.StatusBadRequest)
		return nil
	}
	this.ReleaseNodeIP(address)
	return nil
}
func (this localControllerDelegate) actionNodeStarted(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	node := Node{}
	bodyBytes, err := ioutil.ReadAll(context.Request.Body)
	if err != nil {
		writer.Write([]byte("No body: " + err.Error()))
		writer.WriteHeader(http.StatusBadRequest)
		return nil
	}
	err = json.Unmarshal(bodyBytes, node)
	switch node.NodeType {
	case Master:
		this.clusterState.Masters[node.Id] = node
	case Worker:
		this.clusterState.Workers[node.Id] = node
	case Controller:
		// nothing todo
	case UndefinedNode:
		fallthrough
	default:
		writer.Write([]byte("Unknown node type: " + err.Error()))
		writer.WriteHeader(http.StatusBadRequest)
		return nil
	}
	writer.WriteHeader(http.StatusOK)
	return nil
}
func (this localControllerDelegate) actionNodeStopped(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	nodeId := context.GetQueryParameter("id")
	node := this.clusterState.getNode(nodeId)
	if node == nil {
		writer.WriteHeader(http.StatusNotFound)
		return nil
	}
	this.clusterState.removeNode(node)
	return nil
}
func (this localControllerDelegate) actionGetMasters(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	data, err := json.MarshalIndent(this.clusterState.Masters, "", "  ")
	if err != nil {
		writer.Write([]byte("Failed to mashal masters: " + err.Error()))
		writer.WriteHeader(http.StatusInternalServerError)
	}
	writer.Write(data)
	writer.Header().Set("Content-Type", "application/json")
	return nil
}
func (this localControllerDelegate) actionGetWorkers(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	data, err := json.MarshalIndent(this.clusterState.Workers, "", "  ")
	if err != nil {
		writer.Write([]byte("Failed to mashal workers: " + err.Error()))
		writer.WriteHeader(http.StatusInternalServerError)
	}
	writer.Write(data)
	writer.Header().Set("Content-Type", "application/json")
	return nil
}

func (this localControllerDelegate) actionMasterInfo(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	if !Container().Config.IsMasterNode() {
		writer.Write([]byte("No master running."))
		writer.WriteHeader(http.StatusNotFound)
	}
	result, returnCode := this.execCommand("vagrant", "status "+Container().Config.WorkerNode.NodeName, *Container().Config.WorkerNode)
	if returnCode == http.StatusOK {
		writer.Write(result)
		writer.Header().Set("Content-Type", "application/json")
	} else {
		writer.Write(result)
		writer.WriteHeader(returnCode)
	}
	return nil
}

func (this localControllerDelegate) actionWorkerInfo(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	if !Container().Config.IsWorkerNode() {
		writer.Write([]byte("No worker running."))
		writer.WriteHeader(http.StatusNotFound)
	}
	result, returnCode := this.execCommand("vagrant", "status "+Container().Config.WorkerNode.NodeName, *Container().Config.WorkerNode)
	if returnCode == http.StatusOK {
		writer.Write(result)
		writer.Header().Set("Content-Type", "application/json")
	} else {
		writer.Write(result)
		writer.WriteHeader(returnCode)
	}
	return nil
}

func (this localControllerDelegate) actionMasterExecCommand(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	command := context.GetParameter("cmd")
	if command == "" {
		writer.Write([]byte("No command passed."))
		writer.WriteHeader(http.StatusBadRequest)
	}
	if Container().Config.IsMasterNode() {
		result, returnCode := this.execCommand("vagrant", "ssh -c "+command, *Container().Config.WorkerNode)
		if returnCode == http.StatusOK {
			writer.Write(result)
			writer.Header().Set("Content-Type", "application/json")
		} else {
			writer.Write(result)
			writer.WriteHeader(returnCode)
		}
		return nil
	} else {
		result := make(map[string]string)
		result["result"] = "No master configured"
		result["command"] = command
		result["exitCode"] = "-1"
		json, _ := json.MarshalIndent(result, "", "  ")
		writer.Write(json)
		writer.Header().Set("Content-Type", "application/json")
		return nil
	}
}

func (this localControllerDelegate) actionWorkerExecCommand(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	command := context.GetParameter("cmd")
	if command == "" {
		writer.Write([]byte("No command passed."))
		writer.WriteHeader(http.StatusBadRequest)
	}
	if Container().Config.IsWorkerNode() {
		result, returnCode := this.execCommand("vagrant", "ssh -c "+command, *Container().Config.WorkerNode)
		if returnCode == http.StatusOK {
			writer.Write(result)
			writer.Header().Set("Content-Type", "application/json")
		} else {
			writer.Write(result)
			writer.WriteHeader(returnCode)
		}
		return nil
	} else {
		result := make(map[string]string)
		result["result"] = "No worker configured"
		result["command"] = command
		result["exitCode"] = "-1"
		json, _ := json.MarshalIndent(result, "", "  ")
		writer.Write(json)
		writer.Header().Set("Content-Type", "application/json")
		return nil
	}
}

func (this localControllerDelegate) execCommand(command string, args string, node ClusterNodeConfig) ([]byte, int) {
	if command == "" {
		return []byte("ERROR: No command passed."), http.StatusBadRequest
	}
	output, exitCode := this.execVagrantCommand("vagrant ssh -c \""+command+"\" "+node.NodeName, node)
	if exitCode != 0 {
		return []byte(output), http.StatusInternalServerError
	}
	result := make(map[string]string)
	result["result"] = output
	result["command"] = command
	result["node"] = node.NodeName
	result["address"] = node.NodeAddress
	result["exitCode"] = strconv.Itoa(exitCode)
	json, _ := json.MarshalIndent(result, "", "  ")
	return json, http.StatusOK
}

func (this localControllerDelegate) execVagrantCommand(command string, node ClusterNodeConfig) (string, int) {
	cmd, cmdReader, err := util.RunCommand("Execute remote command", "vagrant", "ssh", "-c", "\""+command+"\" ", node.NodeName)
	if err != nil {
		return "ERROR: '" + command + "' failed: " + err.Error(), -1
	}
	var buff bytes.Buffer
	scanner := bufio.NewScanner(cmdReader)
	for scanner.Scan() {
		buff.WriteString(scanner.Text())
	}
	return string(buff.Bytes()), cmd.ProcessState.ExitCode()
}

func actionKnownIds(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	data, err := json.MarshalIndent((*Container().LocalController).GetKnownClusters(), "", "  ")
	if err != nil {
		writer.Write([]byte("Failed to serialize known cluster ids to JSON: " + err.Error()))
		writer.WriteHeader(http.StatusInternalServerError)
	} else {
		writer.Write(data)
		writer.Header().Set("Content-Type", "application/json")
	}
	return nil
}

func actionClusterState(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	result := make(map[string]string)
	result["cluster"] = Container().Config.ClusterId()
	result["timestamp"] = time.Now().String()
	result["ClusterState"] = (*Container().LocalController).GetState()
	json, _ := json.MarshalIndent(result, "", "  ")
	writer.Write(json)
	writer.Header().Set("Content-Type", "application/json")
	return nil
}

func actionMasterState(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	if Container().Config.IsMasterNode() {
		_, cmdReader, err := util.RunCommand("Get master status.", "vagrant", "status", Container().Config.MasterNode.NodeName)
		if err != nil {
			writer.Write([]byte("ERROR: vagrant status " + Container().Config.MasterNode.NodeName + "' failed: " + err.Error()))
			writer.WriteHeader(http.StatusInternalServerError)
		}
		var buff bytes.Buffer
		scanner := bufio.NewScanner(cmdReader)
		for scanner.Scan() {
			buff.WriteString(scanner.Text())
		}
		writer.Write(buff.Bytes())
	} else {
		writer.Write([]byte("ERROR: no master present on this node."))
		writer.WriteHeader(http.StatusNotFound)
	}
	return nil
}

func actionWorkerState(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	if Container().Config.IsWorkerNode() {
		_, cmdReader, err := util.RunCommand("Get worker status.", "vagrant", "status", Container().Config.WorkerNode.NodeName)
		if err != nil {
			writer.Write([]byte("ERROR: vagrant status " + Container().Config.WorkerNode.NodeName + "' failed: " + err.Error()))
			writer.WriteHeader(http.StatusInternalServerError)
		}
		var buff bytes.Buffer
		scanner := bufio.NewScanner(cmdReader)
		for scanner.Scan() {
			buff.WriteString(scanner.Text())
		}
		writer.Write(buff.Bytes())
	} else {
		writer.Write([]byte("ERROR: no worker present on this node."))
		writer.WriteHeader(http.StatusNotFound)
	}
	return nil
}

func remove(items []Node, i int) []Node {
	items[len(items)-1], items[i] = items[i], items[len(items)-1]
	return items[:len(items)-1]
}

// utils

func nodeFromService(s netutil.Service) *Node {
	return &Node{
		Id:        s.Id,
		Host:      s.Host(),
		Timestamp: time.Now(),
		NodeType:  getNodeType(s),
		ClusterId: getClusterId(s),
		Endpoint:  s.Location,
		Name:      getNodeName(s),
	}
}

func performGet(uri string) ([]byte, error) {
	resp, err := http.Get(uri)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, readErr := ioutil.ReadAll(resp.Body)
	if readErr != nil {
		return nil, readErr
	}
	return data, nil
}

func performDelete(uri string) ([]byte, error) {
	req, err := http.NewRequest("DELETE", uri, nil)
	if util.CheckAndLogError("Failed to create delkete request", err) {
		resp, qerr := http.DefaultClient.Do(req)
		if qerr != nil {
			return nil, qerr
		}
		return ioutil.ReadAll(resp.Body)
	}
	return nil, err
}

func loadRemoteConfig(connection ClusterControllerConnection) (*ClusterConfig, error) {
	data, err := performGet("http://" + connection.ControllerHost + ":9999/cluster/config")
	if err != nil {
		return nil, err
	}
	config := &ClusterConfig{}
	err = json.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}
	return config, nil
}
