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
 * Server command, whcih starts the communication server to establish the one-to-one communications between masters and
 * nodes.
 */
package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/winkube/service"
	"github.com/winkube/service/netutil"
	"net"
	"net/http"
)

type RegistrationHandler struct {
	answers []string
}

func (RegistrationHandler) MsgReceived(src *net.UDPAddr, message string) {
	var model service.InstanceModel
	json.Unmarshal([]byte(message), model)
	service.GetCluster().RegisterInstance(model)
}

var registrationHandler RegistrationHandler

func main() {
	mc := netutil.GetMulticast()
	registrationHandler = RegistrationHandler{
		answers: []string{},
		//Age: 240,
	}

	go mc.Start(func() string {
		bytes, _ := json.Marshal(service.GetInstanceModel())
		return string(bytes)
	},
		registrationHandler)
	fmt.Println("Starting rest endpoint...")
	r := mux.NewRouter()
	r.HandleFunc("/", HomeHandler)
	r.HandleFunc("/cluster", ClusterHandler)
	r.HandleFunc("/setup", SetupHandler)
	http.Handle("/", r)

	http.ListenAndServe(":8080", nil)
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	bytes, _ := json.Marshal(service.GetInstanceModel())
	w.Write(bytes)
}

func ClusterHandler(w http.ResponseWriter, r *http.Request) {
	bytes, _ := json.Marshal(service.GetCluster())
	w.Write(bytes)
}

func SetupHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Setup..."))
}
