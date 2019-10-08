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

package netutil

import (
	"encoding/json"
	"github.com/koron/go-ssdp"
	"log"
	"net"
	"sync"
	"time"
)

type Service struct {
	AdType   string // service ad type
	Usn      string // unique id
	Location string // adress/IP
	Service  string // service type
	MaxAge   int    // max caching age
}

type ServiceHandler interface {
	ServiceReceived(service Service)
}

type Multicast interface {
	Start(out func() string, messageHandler ServiceHandler)
}

type MulticastInstance struct {
	maxAge          int
	aliveInterval   int
	verbose         bool
	advertiser      *ssdp.Advertiser
	monitor         *ssdp.Monitor
	serviceHandlers []ServiceHandler
	aliveTick       <-chan time.Time
}

var (
	multicastInstance MulticastInstance
	once              sync.Once
)

//const MULTICAST_ADR = "224.0.0.10:9000"

func GetMulticast() MulticastInstance {
	once.Do(func() {
		multicastInstance = MulticastInstance{
			maxAge:        1800,
			aliveInterval: 10,
			verbose:       true,
		}
		// start cient
		multicastInstance.monitor = &ssdp.Monitor{
			Alive:  onAlive,
			Bye:    onBye,
			Search: onSearch,
		}
		if err := multicastInstance.monitor.Start(); err != nil {
			log.Fatal(err)
		}
	})
	return multicastInstance
}

func onAlive(m *ssdp.AliveMessage) {
	s := Service{
		AdType:   m.Type,
		Usn:      m.USN,
		Location: m.Location,
		Service:  m.Server,
		MaxAge:   m.MaxAge(),
	}
	json, _ := json.MarshalIndent(s, "", "  ")
	log.Println("[SSDP] received: " + string(json))
	for _, h := range GetMulticast().serviceHandlers {
		h.ServiceReceived(s)
	}
}

func onBye(m *ssdp.ByeMessage) {
	log.Printf("Bye: From=%s Type=%s USN=%s", m.From.String(), m.Type, m.USN)
}

func onSearch(m *ssdp.SearchMessage) {
	log.Printf("Search: From=%s Type=%s", m.From.String(), m.Type)
}

func (mc *MulticastInstance) listen(listener ServiceHandler) {
	mc.serviceHandlers = append(mc.serviceHandlers, listener)
}

func (mc MulticastInstance) StartAdvertizer(out func() Service) {
	service := out()
	if mc.verbose {
		bytes, err := json.MarshalIndent(service, "", "  ")
		if err == nil {
			log.Println("[SSDP], advertize " + string(bytes))
		}
	}
	var err error
	mc.advertiser, err = ssdp.Advertise(
		service.AdType,
		service.Usn,
		service.Location,
		service.Service,
		service.MaxAge)
	if err != nil {
		log.Fatal(err)
	}
	defer announceAlive(mc.verbose, out)
}

func announceAlive(verbose bool, out func() Service) {
	if verbose {
		log.Println("[SSDP], start keep alive loop...(every 10 seconds)... ")
	}
	for {
		time.Sleep(10 * time.Second)
		service := out()
		err := ssdp.AnnounceAlive(service.AdType, service.Usn, service.Location, service.Service,
			service.MaxAge, service.Location)
		if err != nil {
			log.Fatal(err)
		} else if verbose {
			json, _ := json.MarshalIndent(service, "", "  ")
			log.Println("[SSDP] keep alive sent for: " + string(json))
		}
	}
}

// Evakuates all adresses and takes the first non loopback IP presenbt.
func GetInternalIP() string {
	addresses, _ := net.InterfaceAddrs() //here your interface
	var ip net.IP
	for _, addr := range addresses {
		switch v := addr.(type) {
		case *net.IPNet:
			if !v.IP.IsLoopback() {
				if v.IP.To4() != nil && ip == nil { //Verify if IP is IPV4
					ip = v.IP
				}
			}
		}
	}
	if ip != nil {
		return ip.String()
	} else {
		return ""
	}
}
