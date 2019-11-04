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
	"bytes"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/winkube/webapp"
	"golang.org/x/text/language"
	"net/http"
	"os/exec"
	"time"
)

func MonitorWebApplication(router *mux.Router) *webapp.WebApplication {
	log.Info("Initializing root application (monitor)...")
	monitorWebapp := webapp.CreateWebApp("WinKube-Setup", "/", language.English)
	// Pages
	monitorWebapp.AddPage(&webapp.Page{
		Name:     "index",
		Template: "templates/index.html",
	}).AddPage(&webapp.Page{
		Name:     "actions",
		Template: "templates/actions.html",
	}).AddPage(&webapp.Page{
		Name:     "actions-completed",
		Template: "templates/actions-completed.html",
	}).AddPage(&webapp.Page{
		Name:     "actionlog",
		Template: "templates/action-log.html",
	})
	// Actions
	monitorWebapp.GetAction("/", MainIndexAction)
	monitorWebapp.GetAction("/start", StartAction)
	monitorWebapp.GetAction("/stop", StopAction)
	monitorWebapp.GetAction("/actions", ActionsAction)
	monitorWebapp.GetAction("/actionlog", ActionLogAction)
	monitorWebapp.GetAction("/actions-completed", ActionsCompletedAction)
	monitorWebapp.GetAction("/status", LogNodeStatusAction)
	monitorWebapp.GetAction("/enter-setup", EnterSetupAction)
	monitorWebapp.GetAction("/console", NodeConsoleAction)
	//monitorWebapp.GetAction("/cordon", &NodeCordonAction{})
	//monitorWebapp.GetAction("/drain", &NodeDrainAction{})
	return monitorWebapp
}

type NodeInfo struct {
}

type ClusterInfo struct {
}

type Info struct {
	NodeInfo    NodeInfo
	ClusterInfo ClusterInfo
}

func ActionsAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	data := make(map[string]interface{})
	data["actions"] = (*GetActionManager()).RunningActions()
	return &webapp.ActionResponse{
		NextPage: "actions",
		Model:    data,
	}
}

func ActionsCompletedAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	data := make(map[string]interface{})
	data["actions"] = (*GetActionManager()).CompletedActions()
	return &webapp.ActionResponse{
		NextPage: "actions-completed",
		Model:    data,
	}
}

func ActionLogAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	data := make(map[string]interface{})
	actionId := context.GetQueryParameter("actionId")
	backAction := context.GetQueryParameter("backAction")
	action := (*GetActionManager()).LookupAction(actionId)
	data["Action"] = action
	data["backAction"] = backAction
	return &webapp.ActionResponse{
		NextPage: "actionlog",
		Model:    data,
	}
}

// Web action starting the setup process
func MainIndexAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	if !Container().Config.Ready() {
		return &webapp.ActionResponse{
			NextPage: "_redirect",
			Model:    "/setup",
		}
	}
	return &webapp.ActionResponse{
		NextPage: "index",
		Model: Info{
			NodeInfo:    NodeInfo{},
			ClusterInfo: ClusterInfo{},
		},
	}
}

func StartAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	(*Container().NodeManager).StartNodes()
	return &webapp.ActionResponse{
		NextPage: "index",
		Model: Info{
			NodeInfo:    NodeInfo{},
			ClusterInfo: ClusterInfo{},
		},
	}
}

func StopAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	(*Container().NodeManager).StopNodes()
	return &webapp.ActionResponse{
		NextPage: "index",
		Model: Info{
			NodeInfo:    NodeInfo{},
			ClusterInfo: ClusterInfo{},
		},
	}
}

func NodeConsoleAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	cmd := exec.Command("cmd", "/C", "start", "vagrant", "ssh")
	err := cmd.Run()
	if err != nil {
		log.Panic("Cannopt open console...", err)
	}
	return &webapp.ActionResponse{
		NextPage: "_redirect",
		Model:    "/",
	}
}

func EnterSetupAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	Container().RequiredAppStatus = APPSTATE_SETUP
	time.Sleep(10 * time.Second)
	return &webapp.ActionResponse{
		NextPage: "_redirect",
		Model:    "/setup",
	}
}

func LogNodeStatusAction(context *webapp.RequestContext, writer http.ResponseWriter) *webapp.ActionResponse {
	cmd := exec.Command("vagrant", "status")
	reader, err := cmd.StdoutPipe()
	if err != nil {
		log.Panic("Cannopt open console...", err)
	}
	var buff = bytes.Buffer{}
	buff.ReadFrom(reader)
	writer.Write([]byte("Status:\n"))
	writer.Write(buff.Bytes())
	return &webapp.ActionResponse{
		Complete: true,
	}
}
