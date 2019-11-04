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
	"encoding/json"
	"errors"
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
	ClusterId string    `json:"clusterId"`
	NodeType  NodeType  `json:"nodeType"`
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	Timestamp time.Time `json:"timestamp"`
	Endpoint  string    `json:"endpoint"`
}

type Cluster struct {
	ClusterConfig *ClusterConfig `json:"config"`
	Controller    *Node          `json:"controllerConnection"`
	Masters       []Node         `json:"masters"`
	Workers       []Node         `json:"workers"`
}

func (this Cluster) addOrUpdateMaster(node Node) {
	index := util.IndexOf(this.Masters, node)
	if index < 0 {
		this.Masters = append(this.Masters, node)
	} else {
		this.Masters[index] = node
	}
}
func (this Cluster) addOrUpdateWorker(node Node) {
	index := util.IndexOf(this.Workers, node)
	if index < 0 {
		this.Workers = append(this.Workers, node)
	} else {
		this.Workers[index] = node
	}
}

func (this Cluster) removeNode(node *Node) {
	for index, n := range this.Workers {
		if n.Id == node.Id {
			this.Workers = remove(this.Workers, index)
		}
	}
	for index, n := range this.Masters {
		if n.Id == node.Id {
			this.Masters = remove(this.Masters, index)
		}
	}
}

type ControllerDelegate interface {
	Start() error
	Stop() error
	GetClusterId() string
	GetClusterConfig() ClusterConfig
	Exec(command string) string
	ReserveNodeIP(master bool) string
	ReserveInternalIP(master bool) string
	ReleaseNodeIP(string)
	ReleaseInternalIP(string)
}

type LocalController interface {
	IsRunning() bool
	Start(config *SystemConfiguration) error
	Stop() error
	GetClusterId() string
	GetClusterConfig() ClusterConfig
	GetState() string
	ReserveNodeIP(master bool) string
	ReserveInternalIP(master bool) string
	ReleaseNodeIP(string)
	ReleaseInternalIP(string)
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

// The cluster manager is the proxy management component which connects this machine with the overall
// controllerConnection. If the controllerConnection is locally, the this component also manages the controllerConnection
// api which is used by other nodes.
type localController struct {
	serviceRegistry    *netutil.ServiceRegistry `validate:"required"`
	controllerDelegate *ControllerDelegate      `validate:"required"`
	clusterId          string                   `validate:"required"`
	knownClusters      map[string]*Cluster
}

func (c *localController) Start(config *SystemConfiguration) error {
	Log().Info("Starting local controller...")
	if config.IsControllerNode() {
		return c.startLocal(*config.ControllerConfig, config.Id)
	} else {
		return c.startRemote(*config.ClusterLogin)
	}
}

func (c *localController) ReserveNodeIP(master bool) string {
	c.ensureRunning()
	return (*c.controllerDelegate).ReserveNodeIP(master)
}

func (c *localController) ReserveInternalIP(master bool) string {
	c.ensureRunning()
	return (*c.controllerDelegate).ReserveInternalIP(master)
}

func (c *localController) ReleaseNodeIP(ip string) {
	c.ensureRunning()
	(*c.controllerDelegate).ReleaseNodeIP(ip)
}

func (c *localController) ReleaseInternalIP(ip string) {
	c.ensureRunning()
	(*c.controllerDelegate).ReleaseInternalIP(ip)
}

// Checks if this component is running, panics otherwise
func (c *localController) ensureRunning() {
	if !c.IsRunning() {
		panic("Local controller is not running")
	}
}

func (c *localController) DrainNode(node Node) {
	panic("implement me")
}

func (c *localController) CordonNode(node Node) {
	panic("implement me")
}

func (c *localController) IsRunning() bool {
	return c.controllerDelegate != nil
}

func (c *localController) GetState() string {
	return (*c.controllerDelegate).Exec("kubectl get nodes")
}

func (this *localController) startLocal(config ClusterConfig, nodedId string) error {
	Log().Info("Starting local cluster controller for cluster: " + config.ClusterId)
	this.clusterId = config.ClusterId
	clusterState := this.knownClusters[config.ClusterId]
	if clusterState == nil {
		clusterState = &Cluster{
			ClusterConfig: &config,
			Controller:    createLocalControllerNode(config.ClusterId, nodedId),
			Masters:       []Node{},
			Workers:       []Node{},
		}
		this.knownClusters[config.ClusterId] = clusterState
	} else {
		if clusterState.Controller.Host != hostname() {
			panic(fmt.Sprint("Cluster is remotedly managed. Cannot start a local controllerConnection for &v", config.ClusterId))
		}
	}
	clController := localControllerDelegate{
		clusterState:   clusterState,
		clusterNetCIDR: netutil.CreateCIDR(config.ClusterNetCIDR),
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

func (this *localController) GetClusterByName(clusterName string) *Cluster {
	return this.knownClusters[clusterName]
}

func (this *localController) UpdateAndGetClusterByName(clusterName string) *Cluster {
	// TODO perform update
	return this.GetClusterByName(clusterName)
}

func (this *localController) updateService(service netutil.Service) error {
	clusterId := getClusterId(service)
	cluster, found := this.knownClusters[clusterId]
	if !found {
		return errors.New("Cluster not found: " + clusterId)
	}
	cluster.registerService(service)
	return nil
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
	return r.masterRemoteExec("kubectl get nodes")
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

func (r remoteControllerDelegate) ReserveInternalIP(master bool) string {
	// Call controllerConnection to get internal ip
	data, err := performGet("http://" + r.controllerConnection.ControllerHost + ":9999/cluster/internalip?master=" + strconv.FormatBool(master))
	if err != nil {
		Log().Error("ReserveInternalIP", err)
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

func (r remoteControllerDelegate) ReleaseInternalIP(ip string) {
	// Call controllerConnection to relöease internal ip
	_, err := performDelete("http://" + r.controllerConnection.ControllerHost + ":9999/cluster/internalip?ip=" + ip)
	if err != nil {
		Log().Error("ReleaseInternalIP", err)
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
// nodes (masters and workers) in the knownClusters, the IP addresses used for bridge nodes (VMNetCIDR) as
// well as for internal NAT addressing (internalNetCIDR) and finally the credentials for joining
// the cluster.
type localControllerDelegate struct {
	clusterState   *Cluster `validate:"required"`
	clusterNATCIDR *netutil.CIDR
	clusterNetCIDR *netutil.CIDR
	server         *http.Server
}

func (r *localControllerDelegate) GetState() string {
	return r.Exec("kubectl get nodes")
}

func (c *localControllerDelegate) Start() error {
	// start the cloud server
	Log().Info("Initializing controllerConnection api...")
	router := mux.NewRouter()
	clusterApiApp := createClusterManagerWebApp(c)
	router.PathPrefix("/cluster").HandlerFunc(clusterApiApp.HandleRequest)
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
	return c.clusterState.Masters
}

func (c *localControllerDelegate) GetWorkers() []Node {
	return c.clusterState.Workers
}

func (c *localControllerDelegate) ReserveNodeIP(master bool) string {
	if c.clusterNATCIDR == nil {
		return ""
	}
	return (*c.clusterNetCIDR).GetFreeIp()
}

func (c *localControllerDelegate) ReserveInternalIP(master bool) string {
	if c.clusterNATCIDR == nil {
		return ""
	}
	return (*c.clusterNATCIDR).GetFreeIp()
}

func (c *localControllerDelegate) ReleaseNodeIP(ip string) {
	if c.clusterNetCIDR == nil {
		return
	}
	(*c.clusterNetCIDR).MarkIpUnused(ip)
}

func (c *localControllerDelegate) ReleaseInternalIP(ip string) {
	if c.clusterNATCIDR == nil {
		return
	}
	(*c.clusterNATCIDR).MarkIpUnused(ip)
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
	// format: service:clusterId
	return strings.TrimLeft(service.Service, ":")
}

// Evaluates the service id as the left part of the WinKube service identifier:
// e.g. 'master:myCluster01' results in 'master'
func getServiceIdentifier(service netutil.Service) string {
	// format: service:clusterId
	return strings.TrimRight(service.Service, ":")
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

func (this Cluster) registerService(service netutil.Service) {
	//// check, if already registered as node
	Log().Debug("Checking if instance is a node...", service)
	//for _, node := range this.Instances {
	//	if service.Location == node.Host {
	//		Container().Logger.Debug("Model is a known node: " + node.Name + "(" + node.Host + ")")
	//		return
	//	}
	//}
	//// check, if already registered as master
	//Log().Debug("Checking if instance is a master...")
	//for _, master := range this.Masters {
	//	if service.Location == master.Host {
	//		Container().Logger.Debug("Model is a known master: " + master.Name + "(" + master.Host + ")")
	//		return
	//	}
	//}
	//// add node to instance list, if not present.
	//Log().Debug("Discovered new service: " + service.Service + "(" + service.Location + ")")
	//existing := this.getNode(&service)
	//if existing == nil {
	//	Log().Debug("Adding service to service catalogue: %v...", service)
	//	this.Instances = append(this.Instances, *Instance_fromService(service))
	//} else {
	//	updateInstance(existing, &service)
	//}
}

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

// This creates the cluster API application serving cluster data to other nodes.
// This application is active only, if this node is configured as a cluster
// controllerConnection.
func createClusterManagerWebApp(controller *localControllerDelegate) *webapp.WebApplication {
	webapp := webapp.CreateWebApp("cluster", "/cluster", language.English)
	webapp.GetAction("/id", controller.actionClusterId)
	webapp.GetAction("/catalog/knownids", actionKnownIds)
	webapp.GetAction("/catalog/state", actionClusterState)
	webapp.GetAction("/config", controller.actionServeClusterConfig)
	webapp.GetAction("/nodeip", controller.actionReserveNodeIP)
	webapp.DeleteAction("/nodeip", controller.actionReleaseNodeIP)
	webapp.GetAction("/internalip", controller.actionReserveInternalIP)
	webapp.DeleteAction("/internalip", controller.actionReleaseInternalIP)
	webapp.PostAction("/node", controller.actionNodeStarted)
	webapp.DeleteAction("/node", controller.actionNodeStopped)
	webapp.DeleteAction("/masters", controller.actionGetMasters)
	webapp.DeleteAction("/workers", controller.actionGetWorkers)
	return webapp
}

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

func (this localControllerDelegate) actionReserveInternalIP(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	master := util.ParseBool(context.GetQueryParameter("master"))
	ip := this.ReserveInternalIP(master)
	if ip == "" {
		writer.WriteHeader(http.StatusNotFound)
		return nil
	}
	writer.Write([]byte(ip))
	writer.Header().Set("Content-Type", "text/plain")
	writer.WriteHeader(http.StatusOK)
	return nil
}
func (this localControllerDelegate) actionReleaseInternalIP(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	address := context.GetQueryParameter("address")
	if address == "" {
		writer.Write([]byte("Parameter 'address' missing."))
		writer.WriteHeader(http.StatusBadRequest)
		return nil
	}
	this.ReleaseInternalIP(address)
	writer.WriteHeader(http.StatusOK)
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
	writer.WriteHeader(http.StatusOK)
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
	writer.WriteHeader(http.StatusOK)
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
		this.clusterState.addOrUpdateMaster(node)
	case Worker:
		this.clusterState.addOrUpdateWorker(node)
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
	writer.WriteHeader(http.StatusOK)
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
	writer.WriteHeader(http.StatusOK)
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
	writer.WriteHeader(http.StatusOK)
	return nil
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
	clusterId := context.GetQueryParameter("cluster")
	var data []byte
	var err error
	localController := *Container().LocalController
	if clusterId == "" {
		data, err = json.MarshalIndent(localController.GetClusterById(localController.GetClusterId()), "", "  ")
	} else {
		data, err = json.MarshalIndent(localController.GetClusterById(clusterId), "", "  ")
	}
	if err != nil {
		writer.Write([]byte("Failed to serialize cluster state to JSON: " + err.Error()))
		writer.WriteHeader(http.StatusInternalServerError)
	} else {
		writer.Write(data)
		writer.Header().Set("Content-Type", "application/json")
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
