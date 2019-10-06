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

type RegistrationHandler2 struct{
	answers []string;
}

func (RegistrationHandler2) MsgReceived(src *net.UDPAddr, message string){
	log.Println(message)
}

var (
	registrationHandler2 RegistrationHandler2 = RegistrationHandler2{
		answers: []string{},
		//Age: 240,
	}
)

func main() {
	mc := netutil.GetMulticast();

	go mc.Start(func() string {
			bytes, _ := json.Marshal(service.GetInstanceModel())
			return string(bytes)
		},
		registrationHandler2)
	fmt.Println("Starting rest endpoint...")
	http.HandleFunc("/", clusterHandler2)
	http.HandleFunc("/cluster", clusterHandler2)
	http.ListenAndServe(":8081", nil)
}

func clusterHandler2(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Mein Cluster"))
}