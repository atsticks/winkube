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
	id               string              `json:"id"`
	ClusterId        string              `json:"clusterId"`
	NodeType         NodeType            `json:"nodeType"`
	Name             string              `json:"name"`
	Host             string              `json:"host"`
	timestamp        time.Time           `json:"timestamp"`
	ServiceEndpoints map[Endpoint]string `json:"endpoints"`
}

type Cluster struct {
	ClusterConfig *ClusterConfig `json:"config"`
	Controller    *Node          `json:"controller"`
	Masters       []Node         `json:"masters"`
	Workers       []Node         `json:"workers"`
}

func NodeFromService(s netutil.Service) *Node {
	return &Node{
		id:               s.Id,
		Host:             s.Host(),
		timestamp:        time.Now(),
		NodeType:         getNodeType(s),
		ClusterId:        getClusterId(s),
		ServiceEndpoints: map[Endpoint]string{getServiceEndpoint(s): s.Location},
	}
}

type ClusterController interface {
	Start() error
	Stop() error
	GetClusterId() string
	GetClusterConfig() *ClusterConfig
	Exec(command string) string
	ReserveNodeIP(master bool) string
	ReserveInternalIP(master bool) string
	ReleaseNodeIP(string)
	ReleaseInternalIP(string)
}

type ClusterManager interface {
	IsRunning() bool
	StartLocalManager(clusterConfig *ClusterConfig, nodeId string) error
	StartRemoteManager(clusterConfig *ClusterConfig) error
	Stop() error
	GetClusterConfig() *ClusterConfig
	GetState() string
	ReserveNodeIP(master bool) string
	ReserveInternalIP(master bool) string
	ReleaseNodeIP(string)
	ReleaseInternalIP(string)
	DrainNode(node Node)
	CordonNode(node Node)

	GetKnownClusters() []*Cluster
	GetClusterById(clusterId string) *Cluster
	UpdateCluster(clusterId string) *Cluster
}

func CreateClusterManager(serviceRegistry *netutil.ServiceRegistry) *ClusterManager {
	var cm = clusterManager{
		clusters:        make(map[string]*Cluster),
		serviceRegistry: serviceRegistry,
	}
	var CM ClusterManager = &cm
	return &CM
}

func createLocalControllerNode(clusterId string, nodeId string) *Node {
	cn := Node{
		id:               nodeId,
		ClusterId:        clusterId,
		NodeType:         Controller,
		Host:             hostname(),
		timestamp:        time.Now(),
		ServiceEndpoints: map[Endpoint]string{ControllerEndpoint: "http://" + hostname() + ":9999/cluster"},
	}
	return &cn
}

//func createRemoteControllerNode(clusterId string, controller *Node) *ClusterManager {
//	var CM ClusterManager
//	// Load config from remote controller
//	clusterConfig, err := loadRemoteConfig(clusterId, controller)
//	var cm = clusterManager{
//		clusters:        make(map[string]*Cluster),
//		serviceRegistry: serviceRegistry,
//	}
//	var ctl ClusterController = remoteClusterController{
//		controller: controller,
//		config:     clusterConfig,
//		clusterId:  clusterConfig.ClusterId,
//	}
//	cm.clusterController = &ctl
//		err = Container().Validator.Struct(cm)
//	if util.CheckAndLogError("Failed to start cluster manager.", err) {
//		CM = &cm
//		return &CM
//	} else {
//		panic(err)
//	}
//}

// The cluster manager is the proxy management component which connects this machine with the overall
// controller. If the controller is locally, the this component also manages the controller
// api which is used by other nodes.
type clusterManager struct {
	serviceRegistry   *netutil.ServiceRegistry `validate:"required"`
	clusterController *ClusterController       `validate:"required"`
	clusters          map[string]*Cluster
}

func (c clusterManager) isRunning() bool {
	return c.clusterController != nil
}

func (this *clusterManager) StartLocalManager(config *ClusterConfig, nodedId string) error {
	clusterState := &Cluster{
		ClusterConfig: config,
		Controller:    createLocalControllerNode(config.ClusterId, nodedId),
		Masters:       []Node{},
		Workers:       []Node{},
	}
	clController := clusterController{
		clusterState:   clusterState,
		clusterNetCIDR: netutil.CreateCIDR(config.ClusterNetCIDR),
	}
	var cctl ClusterController = clController
	this.clusterController = &cctl
	this.clusters[config.ClusterId] = clusterState
	err := Container().Validator.Struct(this)
	if !util.CheckAndLogError("Failed to start cluster manager.", err) {
		panic(err)
	}
	return clController.Start()
}
func (this *clusterManager) StartRemoteManager(clusterConfig *ClusterConfig) error {
	clusterConfig, err := loadRemoteConfig(clusterConfig.ClusterId, clusterConfig.Controller)
	if !util.CheckAndLogError("Failed to start cluster manager.", err) {
		return err
	}
	clController := remoteClusterController{
		controller: clusterConfig.Controller,
		config:     clusterConfig,
		clusterId:  clusterConfig.ClusterId,
	}
	var cctl ClusterController = clController
	this.clusterController = &cctl
	err = Container().Validator.Struct(this)
	if !util.CheckAndLogError("Failed to start cluster manager.", err) {
		panic(err)
	}
	return clController.Start()
}

func (this *clusterManager) Stop() error {
	if this.clusterController != nil {
		this.clusterController.Stop()
		this.clusterController = nil
	}
	return nil
}

func (this *clusterManager) GetClusterById(clusterId string) *Cluster {
	return this.clusters[clusterId]
}

func (this *clusterManager) UpdateCluster(clusterId string) *Cluster {
	state := this.clusters[clusterId]
	if state != nil {
		if state.ClusterConfig.LocallyManaged {
			return state
		} else {
			// TODO Update Cluster from remote controller
		}
	}
	return state
}

func (this *clusterManager) GetClusterId() string {
	if this.isRunning() {
		return (*this.clusterController).GetClusterId()
	}
	return ""
}
func (this *clusterManager) GetClusterConfig() *ClusterConfig {
	if this.isRunning() {
		return (*this.clusterController).GetClusterConfig()
	}
	return nil
}
func (this *clusterManager) IsController() bool {
	return this.GetClusterConfig().LocallyManaged
}

func (this *clusterManager) GetCluster() *Cluster {
	state := this.GetCluster(this.GetClusterId())
	if state == nil {
		state = &Cluster{
			ClusterConfig: this.clusterConfig,
			Controller:    nil,
			Masters:       []Node{},
			Workers:       []Node{},
		}
		this.clusters[this.GetClusterId()] = state
	}
	return state
}

func (this *clusterManager) GetKnownClusters() []Cluster {
	clusters := []Cluster{}
	for _, v := range this.clusters {
		clusters = append(clusters, v)
	}
	return clusters
}

func (this *clusterManager) GetClusterByName(clusterName string) *Cluster {
	return this.clusters[clusterName]
}

func (this *clusterManager) UpdateAndGetClusterByName(clusterName string) *Cluster {
	// TODO perform update
	return this.GetClusterByName(clusterName)
}

func (this *clusterManager) updateService(service netutil.Service) error {
	clusterId := getClusterId(service)
	cluster, found := this.clusters[clusterId]
	if !found {
		return errors.New("Cluster not found: " + clusterId)
	}
	cluster.registerService(service)
	return nil
}

// A remote ClusterControlPane is an passive management component that delegates cluster management to the
// current active cluster controller, which resideds on another host. It caches and regularly updates
// current cloud configuration from its master controller.
type remoteClusterController struct {
	clusterId  string         `validate:"required"`
	config     *ClusterConfig // will be loaded from the controller...
	controller *Node          `validate:"required"`
}

func (r remoteClusterController) Start() error {
	config, err := loadRemoteConfig(r.clusterId, r.controller)
	if err == nil {
		r.config = config
	}

	return err
}

func (c remoteClusterController) Exec(command string) string {
	// TODO implement remote master exec...
	fmt.Println("TODO implement remote master exec: " + command)
	return "Not implemented: " + command
}

func (r remoteClusterController) Stop() error {
	// nothing to do
	return nil
}

func (r remoteClusterController) GetClusterId() string {
	return r.clusterId
}

func (r remoteClusterController) getConfig() *ClusterConfig {
	if r.config == nil {
		// TODO load config from controller
	}
	return r.config
}

func (r remoteClusterController) IsLocallyManaged() bool {
	return r.getConfig().LocallyManaged
}

func (r remoteClusterController) GetClusterConfig() *ClusterConfig {
	return r.getConfig()
}

func (r remoteClusterController) GetState() string {
	return r.masterRemoteExec("kubectl get nodex")
}

func (r remoteClusterController) GetMasters() []Node {
	// Call controller to get master list
	data, err := performGet(r.controller.ServiceEndpoints[ControllerEndpoint] + "/masters")
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

func (r remoteClusterController) GetWorkers() []Node {
	// Call controller to get worker list
	data, err := performGet(r.controller.ServiceEndpoints[ControllerEndpoint] + "/workers")
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

func (r remoteClusterController) ReserveNodeIP(master bool) string {
	// Call controller to get ip
	data, err := performGet(r.controller.ServiceEndpoints[ControllerEndpoint] + "/nodeip")
	if err != nil {
		Log().Error("ReserveNodeIP", err)
		return ""
	}
	return string(data)
}

func (r remoteClusterController) ReserveInternalIP(master bool) string {
	// Call controller to get internal ip
	data, err := performGet(r.controller.ServiceEndpoints[ControllerEndpoint] + "/internalip")
	if err != nil {
		Log().Error("ReserveInternalIP", err)
		return ""
	}
	return string(data)
}

func (r remoteClusterController) ReleaseNodeIP(string) {
	// Call controller to release ip
	_, err := performDelete(r.controller.ServiceEndpoints[ControllerEndpoint] + "/nodeip")
	if err != nil {
		Log().Error("ReleaseNodeIP", err)
	}
}

func (r remoteClusterController) ReleaseInternalIP(string) {
	// Call controller to relöease internal ip
	_, err := performDelete(r.controller.ServiceEndpoints[ControllerEndpoint] + "/internalip")
	if err != nil {
		Log().Error("ReleaseInternalIP", err)
	}
}

func (r remoteClusterController) DrainNode(node Node) {
	r.masterRemoteExec("kubectl drain " + node.Name)
}

func (r remoteClusterController) CordonNode(node Node) {
	r.masterRemoteExec("kubectl cordon " + node.Name)
}

func (r remoteClusterController) masterRemoteExec(command string) string {
	// Call controller to get internal ip
	data, err := performGet(r.controller.ServiceEndpoints[ControllerEndpoint] + "/exec?cmd=" + command)
	if err != nil {
		Log().Error("Controller failed: EXEC "+command, err)
		return ""
	}
	return string(data)
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

func loadRemoteConfig(id string, controller *Node) (*ClusterConfig, error) {
	data, err := performGet(controller.ServiceEndpoints[ControllerEndpoint] + "/config")
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

// A ClusterControlPane is an active management component that manages a cluster. It trackes the
// nodes (masters and workers) in the clusters, the IP addresses used for bridge nodes (VMNetCIDR) as
// well as for internal NAT addressing (internalNetCIDR) and finally the credentials for joining
// the cluster.
type clusterController struct {
	clusterState   *Cluster `validate:"required"`
	clusterNATCIDR *netutil.CIDR
	clusterNetCIDR *netutil.CIDR
	server         *http.Server
}

func (c clusterController) Start() error {
	// start the cloud server
	Log().Info("Initializing controller api...")
	router := mux.NewRouter()
	clusterApiApp := createClusterManagerWebApp(&c)
	router.PathPrefix("/cluster").HandlerFunc(clusterApiApp.HandleRequest)
	c.server = &http.Server{Addr: "0.0.0.0:9999", Handler: router}
	go c.listenHttp()
	return nil
}

func (c clusterController) listenHttp() {
	c.server.ListenAndServe()
}

func (c clusterController) Stop() error {
	if c.server != nil {
		return c.server.Close()
	}
	return nil
}

func (c clusterController) GetClusterId() string {
	return c.clusterState.ClusterConfig.ClusterId
}

func (c clusterController) IsLocallyManaged() bool {
	return c.clusterState.ClusterConfig.LocallyManaged
}

func (c clusterController) GetClusterConfig() *ClusterConfig {
	return c.clusterState.ClusterConfig
}

func (c clusterController) GetMasters() []Node {
	return c.clusterState.Masters
}

func (c clusterController) GetWorkers() []Node {
	return c.clusterState.Workers
}

func (c clusterController) ReserveNodeIP(master bool) string {
	if c.clusterNATCIDR == nil {
		return ""
	}
	return (*c.clusterNetCIDR).GetFreeIp()
}

func (c clusterController) ReserveInternalIP(master bool) string {
	if c.clusterNATCIDR == nil {
		return ""
	}
	return (*c.clusterNATCIDR).GetFreeIp()
}

func (c clusterController) ReleaseNodeIP(ip string) {
	if c.clusterNetCIDR == nil {
		return
	}
	(*c.clusterNetCIDR).MarkIpUnused(ip)
}

func (c clusterController) ReleaseInternalIP(ip string) {
	if c.clusterNATCIDR == nil {
		return
	}
	(*c.clusterNATCIDR).MarkIpUnused(ip)
}

func (c clusterController) DrainNode(node Node) {
	c.Exec("kubectl drain " + node.Name)
}

func (c clusterController) CordonNode(node Node) {
	c.Exec("kubectl cordon " + node.Name)
}

func (c clusterController) Exec(command string) string {
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

func getServiceEndoint(service netutil.Service) Endpoint {
	service.Service
}

// Evaluates the nodetype based on the current service identifier
func getNodeType(service netutil.Service) NodeType {
	switch getServiceEndoint(service) {
	case MasterEndpoint:
		return Master
	case WorkerEndpoint:
		return Worker
	case ControllerEndpoint:
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
		if serviceId == item.id {
			return &item
		}
	}
	for _, item := range this.Workers {
		if serviceId == item.id {
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
// controller.
func createClusterManagerWebApp(controller *clusterController) *webapp.WebApplication {
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

func (this clusterController) actionClusterId(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	writer.Write([]byte(this.clusterState.ClusterConfig.ClusterId))
	writer.Header().Set("Content-Type", "text/plain")
	writer.WriteHeader(http.StatusOK)
	return nil
}

func actionKnownIds(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	data, err := json.MarshalIndent((*Container().ClusterManager).GetKnownClusters(), "", "  ")
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
	clusterManager := *Container().ClusterManager
	if clusterId == "" {
		data, err = json.MarshalIndent(clusterManager.GetClusterById(clusterManager.GetClusterId()), "", "  ")
	} else {
		data, err = json.MarshalIndent(clusterManager.GetClusterById(clusterId), "", "  ")
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

func (this clusterController) actionServeClusterConfig(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
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

func (this clusterController) actionReserveInternalIP(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
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
func (this clusterController) actionReleaseInternalIP(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
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
func (this clusterController) actionReserveNodeIP(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
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
func (this clusterController) actionReleaseNodeIP(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
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
func (this clusterController) actionNodeStarted(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
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
func (this clusterController) actionNodeStopped(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
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
func (this clusterController) actionGetMasters(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
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
func (this clusterController) actionGetWorkers(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
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
		if n.id == node.id {
			this.Workers = remove(this.Workers, index)
		}
	}
	for index, n := range this.Masters {
		if n.id == node.id {
			this.Masters = remove(this.Masters, index)
		}
	}
}

func remove(items []Node, i int) []Node {
	items[len(items)-1], items[i] = items[i], items[len(items)-1]
	return items[:len(items)-1]
}
