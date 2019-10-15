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
	"errors"
	log "github.com/sirupsen/logrus"
	"github.com/winkube/service/netutil"
	"reflect"
	"time"
)

type Cluster struct {
	clusterConfig ClusterConfig        `json:"clusterConfig",validate:"required"`
	Instances     []RegisteredInstance `json:"instances"`
	Masters       []Master             `json:"masters"`
	Nodes         []Node               `json:"nodes"`
}

type ClusterManager interface {
	GetCluster(clusterId string) *Cluster
	UpdateAndGetCluster(clusterId string) *Cluster
	GetClusterIDs() []string
}

func CreateClusterManager(serviceRegistry *netutil.ServiceRegistry) ClusterManager {
	cm := &clusterManager{
		clusters:        make(map[string]*Cluster),
		serviceRegistry: serviceRegistry,
	}
	return cm
}

type clusterManager struct {
	clusters        map[string]*Cluster
	serviceRegistry *netutil.ServiceRegistry
}

func (this clusterManager) GetClusterIDs() []string {
	keys := []string{}
	for k := range reflect.ValueOf(this.clusters).MapKeys() {
		keys = append(keys, string(k))
	}
	return keys
}

func (this clusterManager) GetCluster(clusterId string) *Cluster {
	return this.clusters[clusterId]
}

func (this clusterManager) UpdateAndGetCluster(clusterId string) *Cluster {
	// TODO perform update
	return this.GetCluster(clusterId)
}

func (this clusterManager) updateService(service netutil.Service) error {
	clusterId := getClusterId(service)
	cluster, found := this.clusters[clusterId]
	if !found {
		return errors.New("Cluster not found: " + clusterId)
	}
	cluster.registerService(service)
	return nil
}

func getClusterId(service netutil.Service) string {
	// TODO getClusterId(Service)
	return "myClusterId"
}

func (this Cluster) registerService(service netutil.Service) {
	// check, if already registered as node
	log.Debug("Checking if instance is a node...")
	for _, node := range this.Nodes {
		if service.Location == node.Host {
			log.Debug("Model is a known node: " + node.Name + "(" + node.Host + ")")
			return
		}
	}
	// check, if already registered as master
	log.Debug("Checking if instance is a master...")
	for _, master := range this.Masters {
		if service.Location == master.Host {
			log.Debug("Model is a known master: " + master.Name + "(" + master.Host + ")")
			return
		}
	}
	// add node to instance list, if not present.
	log.Debug("Discovered new service: " + service.Service + "(" + service.Location + ")")
	existing := this.getInstance(&service)
	if existing == nil {
		log.Debug("Adding service to service catalogue: %v...", service)
		this.Instances = append(this.Instances, *RegisteredInstance_fromService(service))
	} else {
		updateInstance(existing, &service)
	}
}

func (this Cluster) getInstance(service *netutil.Service) *RegisteredInstance {
	for _, item := range this.Instances {
		if service.Id == item.id {
			return &item
		}
	}
	return nil
}

func updateInstance(existing *RegisteredInstance, service *netutil.Service) {
	log.Debug("Updating instance %v with %v.", existing, service)
	existing.Host = service.Host()
	existing.Port = service.Port()
	existing.timestamp = time.Now().Nanosecond() / 1000
}

// Find takes a slice and looks for an element using the given predicate in it.
// If found it will return the item found, otherwise nil.
func findWithPredicate(slice []interface{}, predicate func(interface{}) bool) interface{} {
	for item := range slice {
		if predicate(item) {
			return item
		}
	}
	return nil
}
