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

/**
 * Setup Application registered under /setup on startup if no valid config is present.
 */
package service

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/winkube/util"
	"github.com/winkube/webapp"
	"golang.org/x/text/language"
	"net/http"
)

func ClusterWebApplication(router *mux.Router) *webapp.WebApplication {
	Log().Info("Initializing cluster endpoint...")
	setupWebapp := webapp.CreateWebApp("WinKube-Cluster", "/cluster", language.English)
	// Actions
	setupWebapp.GetAction("/Nodes", NodesAction)
	setupWebapp.GetAction("/config", ConfigAction)
	//setupWebapp.GetAction("/config/ip/used", GetUsedIPAction)
	//setupWebapp.GetAction("/config/ip/free", GetFreeIPAction)
	//setupWebapp.GetAction("/config/ip/all", GetAllIPAction)
	return setupWebapp
}

func NodesAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	// Collect messages
	var nodes []*ClusterNodeConfig
	if Container().Config.MasterNode != nil {
		nodes = append(nodes, Container().Config.MasterNode)
	}
	if Container().Config.MasterNode != nil {
		nodes = append(nodes, Container().Config.WorkerNode)
	}
	data, err := json.Marshal(nodes)
	if err == nil {
		writer.Write(data)
		writer.WriteHeader(http.StatusOK)
	} else {
		writer.WriteHeader(http.StatusInternalServerError)
	}
	return &webapp.ActionResponse{
		Complete: true,
	}
}

func ConfigAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	// Collect messages
	config := Container().Config
	data, err := json.Marshal(config)
	util.CheckAndLogError("Cannot create Cluster Record", err)
	writer.Write(data)
	return &webapp.ActionResponse{
		Complete: true,
	}
}

//func GetUsedIPAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
//	// Collect messages
//	config := Container().Config
//	switch(config.NodeType){
//	case Master:
//		cm := *Container().LocalController
//		ip := (*cm.GetClusterControl()).GetNewNodeIp()
//		writer.Write([]byte(ip))
//	default:
//		writer.Write([]byte("Not a master!"))
//		writer.WriteHeader(http.StatusNotImplemented)
//	}
//	return &webapp.ActionResponse{
//		Complete: true,
//	}
//}

//func GetNewIPAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
//	// Collect messages
//	config := Container().Config
//	switch(config.NodeType){
//	case Master:
//		cm := *Container().LocalController
//		ip := (*cm.GetClusterControl()).GetNewNodeIp()
//		writer.Write([]byte(ip))
//	default:
//		writer.Write([]byte("Not a master!"))
//		writer.WriteHeader(http.StatusNotImplemented)
//	}
//	return &webapp.ActionResponse{
//		Complete: true,
//	}
//}
