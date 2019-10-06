/**
 * Server command, whcih starts the communication server to establish the one-to-one communications between masters and
 * nodes.
 */
package main

import (
	"encoding/json"
	"fmt"
	"github.com/winkube/service"
	"github.com/winkube/service/netutil"
	"log"
	"net"
	"net/http"
)

type RegistrationHandler struct{
	answers []string;
}

func (RegistrationHandler) MsgReceived(src *net.UDPAddr, message string){
	log.Println(message)
}

var registrationHandler RegistrationHandler

func main() {
	mc := netutil.GetMulticast();
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
	http.HandleFunc("/", clusterHandler)
	http.HandleFunc("/cluster", clusterHandler)
	http.ListenAndServe(":8080", nil)
}

func clusterHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Mein Cluster"))
}