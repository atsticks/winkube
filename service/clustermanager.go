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
	"reflect"
	"strings"
	"time"
)

type ServiceEndpoint struct {
	Service  string `json:"service"`
	Location string `json:"location"`
}

type Instance struct {
	id        string    `json:"id"`
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	timestamp time.Time `json:"timestamp"`
}

type Node struct {
	Instance         `json:"instance"`
	NodeType         NodeType          `json:"nodeType"`
	ClusterId        string            `json:"clusterId"`
	ServiceEndpoints []ServiceEndpoint `json:"endpoints"`
	NodeConfig       `json:"node"`
}

type ControllerNode struct {
	Instance        `json:"instance"`
	ServiceEndpoint ServiceEndpoint `json:"endpoint"`
	ClusterId       string          `json:"clusterId"`
}

type ClusterState struct {
	ClusterConfig *ClusterConfig  `json:"config"`
	Controller    *ControllerNode `json:"controller"`
	Masters       []Node          `json:"masters"`
	Workers       []Node          `json:"workers"`
}

func CreateLocalInstance(nodeId string) *Instance {
	return &Instance{
		Name: hostname(),
		Host: netutil.GetDefaultIP().String(),
		id:   nodeId,
	}
}

func NodeFromService(s netutil.Service) *Node {
	return &Node{
		Instance: Instance{
			id:        s.Id,
			Host:      s.Host(),
			timestamp: time.Now(),
		},
		NodeType:  getNodeType(s),
		ClusterId: getClusterId(s),
		ServiceEndpoints: []ServiceEndpoint{
			ServiceEndpoint{
				Service:  getServiceIdentifier(s),
				Location: s.Location,
			},
		},
		NodeConfig: NodeConfig{
			NodeType: getNodeType(s),
			NodeIP:   s.Server,
		},
	}
}

type ClusterController interface {
	Start() error
	Stop() error
	GetClusterId() string
	IsLocallyManaged() bool
	GetClusterConfig() ClusterConfig
	GetState() string
	GetMasters() []Node
	GetWorkers() []Node
	ReserveNodeIP(master bool) string
	ReleaseNodeIP(string)
	DrainNode(node Node)
	CordonNode(node Node)
}

type ClusterManager interface {
	StartLocalController(clusterConfig *ClusterConfig, nodeId string) error
	StartRemoteController(clusterConfig *ClusterConfig) error
	Stop() error
	GetClusterId() string
	GetClusterConfig() ClusterConfig
	GetController() *ClusterController
	GetClusterState(clusterId string) *ClusterState
	UpdateClusterState(clusterId string) *ClusterState
	GetKnownClusterIDs() []string
}

func CreateClusterManager(serviceRegistry *netutil.ServiceRegistry) *ClusterManager {
	var cm = clusterManager{
		clusters:        make(map[string]*ClusterState),
		serviceRegistry: serviceRegistry,
	}
	var CM ClusterManager = &cm
	return &CM
}

func createLocalControllerNode(clusterId string, nodeId string) *ControllerNode {
	cn := ControllerNode{
		Instance: *CreateLocalInstance(nodeId),
		ServiceEndpoint: ServiceEndpoint{
			Service:  "ClusterControl",
			Location: "http://" + hostname() + ":9999/cluster",
		},
		ClusterId: clusterId,
	}
	return &cn
}

func CreateDelegatingClusterManager(clusterId string, controller *ControllerNode, serviceRegistry *netutil.ServiceRegistry) *ClusterManager {
	var CM ClusterManager
	// Load config from remote controller
	clusterConfig, err := loadRemoteConfig(clusterId, controller)
	var cm = clusterManager{
		clusters:        make(map[string]*ClusterState),
		clusterConfig:   *clusterConfig,
		serviceRegistry: serviceRegistry,
	}
	cm.clusterController = remoteClusterController{
		controller: controller,
		config:     clusterConfig,
		clusterId:  clusterConfig.ClusterId,
	}
	err = Container().Validator.Struct(cm)
	if util.CheckAndLogError("Failed to start cluster manager.", err) {
		CM = &cm
		return &CM
	} else {
		panic(err)
	}
}

// The cluster manager is the proxy management component which connects this machine with the overall
// controller. If the controller is locally, the this component also manages the controller
// api which is used by other nodes.
type clusterManager struct {
	serviceRegistry   *netutil.ServiceRegistry `validate:"required"`
	clusterConfig     ClusterConfig            `validate:"required"`
	clusterController ClusterController        `validate:"required"`
	clusters          map[string]*ClusterState
	Running           bool
}

// A remote ClusterControlPane is an passive management component that delegates cluster management to the
// current active cluster controller, which resideds on another host. It caches and regularly updates
// current cloud configuration from its master controller.
type remoteClusterController struct {
	clusterId  string          `validate:"required"`
	config     *ClusterConfig  // will be loaded from the controller...
	controller *ControllerNode `validate:"required"`
}

func (r remoteClusterController) Start() error {
	config, err := loadRemoteConfig(r.clusterId, r.controller)
	if err == nil {
		r.config = config
	}

	return err
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

func loadRemoteConfig(id string, controller *ControllerNode) (*ClusterConfig, error) {
	data, err := performGet(controller.ServiceEndpoint.Location + "/cluster/config")
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

func (r remoteClusterController) Stop() error {
	// nothing to do
	return nil
}

func (r remoteClusterController) GetClusterId() string {
	return r.clusterId
}

func (r remoteClusterController) getConfig() ClusterConfig {
	if r.config == nil {
		// TODO load config from controller
	}
	return *r.config
}

func (r remoteClusterController) IsLocallyManaged() bool {
	return r.getConfig().LocallyManaged
}

func (r remoteClusterController) GetClusterConfig() ClusterConfig {
	return r.getConfig()
}

func (r remoteClusterController) GetState() string {
	return r.masterRemoteExec("kubectl get nodex")
}

func (r remoteClusterController) GetMasters() []Node {
	// Call controller to get master list
	data, err := performGet(r.controller.ServiceEndpoint.Location + "/cluster/masters")
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
	data, err := performGet(r.controller.ServiceEndpoint.Location + "/cluster/workers")
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
	data, err := performGet(r.controller.ServiceEndpoint.Location + "/cluster/nodeip")
	if err != nil {
		Log().Error("ReserveNodeIP", err)
		return ""
	}
	return string(data)
}

func (r remoteClusterController) ReserveInternalIP(master bool) string {
	// Call controller to get internal ip
	data, err := performGet(r.controller.ServiceEndpoint.Location + "/cluster/internalip")
	if err != nil {
		Log().Error("ReserveInternalIP", err)
		return ""
	}
	return string(data)
}

func (r remoteClusterController) ReleaseNodeIP(string) {
	// Call controller to release ip
	_, err := performDelete(r.controller.ServiceEndpoint.Location + "/cluster/nodeip")
	if err != nil {
		Log().Error("ReleaseNodeIP", err)
	}
}

func (r remoteClusterController) ReleaseInternalIP(string) {
	// Call controller to relöease internal ip
	_, err := performDelete(r.controller.ServiceEndpoint.Location + "/cluster/internalip")
	if err != nil {
		Log().Error("ReleaseInternalIP", err)
	}
}

func (r remoteClusterController) DrainNode(node Node) {
	r.masterRemoteExec("kubectl drain " + node.NodeName)
}

func (r remoteClusterController) CordonNode(node Node) {
	r.masterRemoteExec("kubectl cordon " + node.NodeName)
}

func (r remoteClusterController) masterRemoteExec(command string) string {
	// Call controller to get internal ip
	data, err := performGet(r.controller.ServiceEndpoint.Location + "/cluster/exec?cmd=" + command)
	if err != nil {
		Log().Error("Controller failed: EXEC "+command, err)
		return ""
	}
	return string(data)
}

// A ClusterControlPane is an active management component that manages a cluster. It trackes the
// nodes (masters and workers) in the clusters, the IP addresses used for bridge nodes (VMNetCIDR) as
// well as for internal NAT addressing (internalNetCIDR) and finally the credentials for joining
// the cluster.
type clusterController struct {
	controller     *ControllerNode `validate:"required"`
	config         *ClusterConfig  `validate:"required"`
	masters        []Node
	workers        []Node
	clusterNetCIDR *netutil.CIDR
	server         *http.Server
}

func (c clusterController) Start() error {
	// Start the cloud server
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
	return c.config.ClusterId
}

func (c clusterController) IsLocallyManaged() bool {
	return c.config.LocallyManaged
}

func (c clusterController) GetClusterConfig() ClusterConfig {
	return *c.config
}

func (c clusterController) GetState() string {
	return c.masterExec("kubectl get nodex")
}

func (c clusterController) GetMasters() []Node {
	return c.masters
}

func (c clusterController) GetWorkers() []Node {
	return c.workers
}

func (c clusterController) ReserveNodeIP(master bool) string {
	if c.clusterNetCIDR == nil {
		return ""
	}
	return (*c.clusterNetCIDR).GetFreeIp()
}

func (c clusterController) ReleaseNodeIP(ip string) {
	if c.clusterNetCIDR == nil {
		return
	}
	(*c.clusterNetCIDR).MarkIpUnused(ip)
}

func (c clusterController) DrainNode(node Node) {
	c.masterExec("kubectl drain " + node.NodeName)
}

func (c clusterController) CordonNode(node Node) {
	c.masterExec("kubectl cordon " + node.NodeName)
}

func (c clusterController) masterExec(command string) string {
	// TODO implement remote master exec...
	fmt.Println("TODO implement remote master exec: " + command)
	return "Not implemented: " + command
}

func (this *clusterManager) StartLocalController(config *ClusterConfig, nodeId string) error {
	action := (*GetActionManager()).StartAction("Staring Local Cluster Controller for cluster " + config.ClusterId)
	defer action.Complete()

	action.LogActionLn("Creating controller node config...")
	localController := createLocalControllerNode(config.ClusterId, nodeId)
	action.LogActionLn("Creating cluster controller...")
	this.clusterController = clusterController{
		controller:     localController,
		config:         config,
		masters:        []Node{},
		workers:        []Node{},
		clusterNetCIDR: netutil.CreateCIDR(config.ClusterNetCIDR),
	}
	action.LogActionLn("Validating cluster manager...")
	err := Container().Validator.Struct(this)
	if !util.CheckAndLogError("Failed to start cluster manager.", err) {
		panic(err)
	}
	config.Controller = localController
	action.LogActionLn("Starting cluster controller...")
	defer action.LogActionLn("Cluster manager running.")
	return this.clusterController.Start()
}
func (this *clusterManager) StartRemoteController(clusterConfig *ClusterConfig) error {
	clusterConfig, err := loadRemoteConfig(clusterConfig.ClusterId, clusterConfig.Controller)
	if !util.CheckAndLogError("Failed to start cluster manager.", err) {
		return err
	}
	this.clusterController = remoteClusterController{
		controller: clusterConfig.Controller,
		config:     clusterConfig,
		clusterId:  clusterConfig.ClusterId,
	}
	err = Container().Validator.Struct(this)
	if !util.CheckAndLogError("Failed to start cluster manager.", err) {
		panic(err)
	}
	return this.clusterController.Start()
}

func (this *clusterManager) Stop() error {
	this.Running = false
	return nil
}

func (this *clusterManager) GetController() *ClusterController {
	var cctl ClusterController = this.clusterController
	return &cctl
}

func (this *clusterManager) GetClusterState(clusterId string) *ClusterState {
	return this.clusters[clusterId]
}

func (this *clusterManager) UpdateClusterState(clusterId string) *ClusterState {
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
	return this.clusterConfig.ClusterId
}
func (this *clusterManager) GetClusterConfig() ClusterConfig {
	return this.clusterConfig
}
func (this *clusterManager) IsController() bool {
	return this.clusterConfig.LocallyManaged
}
func (this *clusterManager) GetCluster() *ClusterState {
	state := this.GetClusterSpec(this.GetClusterId())
	if state == nil {
		state = &ClusterState{
			ClusterConfig: &this.clusterConfig,
			Controller:    nil,
			Masters:       []Node{},
			Workers:       []Node{},
		}
		this.clusters[this.GetClusterId()] = state
	}
	return state
}

func (this *clusterManager) GetKnownClusterIDs() []string {
	keys := []string{}
	for k := range reflect.ValueOf(this.clusters).MapKeys() {
		keys = append(keys, string(k))
	}
	return keys
}

func (this *clusterManager) GetClusterSpec(clusterId string) *ClusterState {
	return this.clusters[clusterId]
}

func (this *clusterManager) UpdateAndGetClusterSpec(clusterId string) *ClusterState {
	// TODO perform update
	return this.GetClusterSpec(clusterId)
}

func (this *clusterManager) updateService(service netutil.Service) error {
	clusterId := getClusterId(service)
	cluster, found := this.clusters[clusterId]
	if !found {
		return errors.New("ClusterState not found: " + clusterId)
	}
	cluster.registerService(service)
	return nil
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

func (this ClusterState) registerService(service netutil.Service) {
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

func (this ClusterState) getNode(service *netutil.Service) *Node {
	for _, item := range this.Masters {
		if service.Id == item.id {
			return &item
		}
	}
	for _, item := range this.Workers {
		if service.Id == item.id {
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
	webapp.GetAction("/config", controller.ActionServeClusterConfig)
	webapp.GetAction("/nodeip", controller.ActionReserveNodeIP)    // GET
	webapp.DeleteAction("/nodeip", controller.ActionReleaseNodeIP) // DELETE
	webapp.PostAction("/node", controller.ActionNodeStarted)
	webapp.DeleteAction("/node", controller.ActionNodeStopped)
	webapp.GetAction("/masters", controller.ActionGetMasters)
	webapp.GetAction("/workers", controller.ActionGetWorkers)
	return webapp
}

func (this clusterController) ActionServeClusterConfig(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	data, err := json.MarshalIndent(this.config, "", "  ")
	if err != nil {
		writer.Write([]byte("Failed to serialize config to JSON: " + err.Error()))
		writer.WriteHeader(http.StatusInternalServerError)
	} else {
		writer.Write(data)
		writer.Header().Set("Content-Type", "application/json")
	}
	return nil
}

func (this clusterController) ActionReserveNodeIP(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	master := util.ParseBool(context.GetQueryParameter("master"))
	ip := this.ReserveNodeIP(master)
	if ip == "" {
		writer.WriteHeader(http.StatusNotFound)
		return nil
	}
	writer.Write([]byte(ip))
	writer.WriteHeader(http.StatusOK)
	writer.Header().Set("Content-Type", "text/plain")
	return nil
}
func (this clusterController) ActionReleaseNodeIP(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
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
func (this clusterController) ActionNodeStarted(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
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
		this.addOrUpdateMaster(node)
	case Worker:
		this.addOrUpdateWorker(node)
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
func (this clusterController) ActionNodeStopped(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	nodeId := context.GetQueryParameter("id")
	node := this.getNode(nodeId)
	if node == nil {
		writer.WriteHeader(http.StatusNotFound)
		return nil
	}
	this.removeNode(node)
	writer.WriteHeader(http.StatusOK)
	return nil
}
func (this clusterController) ActionGetMasters(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	data, err := json.MarshalIndent(this.masters, "", "  ")
	if err != nil {
		writer.Write([]byte("Failed to mashal masters: " + err.Error()))
		writer.WriteHeader(http.StatusInternalServerError)
	}
	writer.Write(data)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	return nil
}
func (this clusterController) ActionGetWorkers(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	data, err := json.MarshalIndent(this.workers, "", "  ")
	if err != nil {
		writer.Write([]byte("Failed to mashal workers: " + err.Error()))
		writer.WriteHeader(http.StatusInternalServerError)
	}
	writer.Write(data)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	return nil
}

func (this clusterController) addOrUpdateMaster(node Node) {
	index := util.IndexOf(this.masters, node)
	if index < 0 {
		this.masters = append(this.masters, node)
	} else {
		this.masters[index] = node
	}
}
func (this clusterController) addOrUpdateWorker(node Node) {
	index := util.IndexOf(this.workers, node)
	if index < 0 {
		this.workers = append(this.workers, node)
	} else {
		this.workers[index] = node
	}
}

func (this clusterController) getNode(id string) *Node {
	for _, n := range this.workers {
		if n.id == id {
			return &n
		}
	}
	for _, n := range this.masters {
		if n.id == id {
			return &n
		}
	}
	return nil
}

func (this clusterController) removeNode(node *Node) {
	for index, n := range this.workers {
		if n.id == node.id {
			this.workers = remove(this.workers, index)
		}
	}
	for index, n := range this.masters {
		if n.id == node.id {
			this.masters = remove(this.masters, index)
		}
	}
}

func remove(items []Node, i int) []Node {
	items[len(items)-1], items[i] = items[i], items[len(items)-1]
	return items[:len(items)-1]
}
