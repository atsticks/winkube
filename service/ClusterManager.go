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
	"github.com/winkube/service/netutil"
	"log"
	"sync"
)

var (
	clusterInstance Cluster
	clusterOnce     sync.Once
)

func GetCluster() Cluster {
	clusterOnce.Do(func() {
		clusterInstance = Cluster{
			IsUseMulticast: true,
			Instances:      []netutil.Service{},
			Masters:        []Master{},
			Nodes:          []Node{},
			ClusterId:      "noid",
		}
	})
	return clusterInstance
}

type Master struct {
	InstanceModel
	Labels map[string]string `json:"labels"`
}

type Node struct {
	InstanceModel
	Labels map[string]string `json:"labels"`
}

type Cluster struct {
	IsUseMulticast     bool              `json:"multicast"`
	Instances          []netutil.Service `json:"instances"`
	Masters            []Master          `json:"masters"`
	Nodes              []Node            `json:"nodes"`
	NodeCidr           string            `json:"nodecidr"`
	ClusterId          string            `json:"clusterID"`
	ClusterCredentials string            `json:"clusterCredentials"`
	ClusterNetwork     string            `json:"clusterNet"`
	Gateway            string            `json:"gateway"`
}

func (cl Cluster) registerService(service netutil.Service) {
	if service.AdType == "winkube-service" {
		log.Println("Discovered new service: " + service.Service + "(" + service.Location + ")")
	}

}

func (cl Cluster) RegisterService(service netutil.Service) {
	// check, if already registered as node
	log.Println("Checking if instance is a node...")
	for _, node := range cl.Nodes {
		if service.Location == node.Host {
			log.Println("Model is a known node: " + node.Name + "(" + node.Host + ")")
			return
		}
	}
	// check, if already registered as master
	log.Println("Checking if instance is a master...")
	for _, master := range cl.Masters {
		if service.Location == master.Host {
			log.Println("Model is a known master: " + master.Name + "(" + master.Host + ")")
			return
		}
	}
	// add node to instance list, if not present.
	cl.registerService(service)
}
