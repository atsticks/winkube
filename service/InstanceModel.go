package service

import (
	"github.com/winkube/service/netutil"
	"os"
	"sync"
)

const UNDEFINED_ROLE string = "undefined";
const MASTER_ROLE string = "master";
const NODE_ROLE string = "node";

var (
	instance InstanceModel
	once sync.Once
)

type InstanceModel struct {
	InstanceRole string `json:"role"`
	Hostname string     `json:"hostname"`
	Aliases []string    `json:"aliases"`
	Ip string           `json:"ip"`
}

type Master struct {
	InstanceModel
	Labels map[string]string  `json:"labels"`
}

type Node struct {
	InstanceModel
	Labels map[string]string  `json:"labels"`
}

type Cluster struct {
	IsUseMulticast bool `json:"multicast"`
	Masters []Master    `json:"masters"`
	Nodes []Node        `json:"nodes"`
	NodeCidr string     `json:"nodecidr"`
	ClusterId string    `json:"clusterID"`
	ClusterCredentials string  `json:"clusterCredentials"`
	ClusterNetwork string  `json:"clusterNet"`
	Gateway string      `json:"gateway"`
}

func GetInstanceModel() InstanceModel{
	once.Do(func() {
		instance = InstanceModel{
			InstanceRole: UNDEFINED_ROLE,
			Hostname: hostname(),
			Aliases:  nil,
			Ip:       netutil.GetInternalIP(),
		}
	})
	return instance
}

func hostname()string{
	var hn string
	hn,_ = os.Hostname()
	return hn
}