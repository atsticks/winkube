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
	log "github.com/sirupsen/logrus"
	"github.com/winkube/service/util"
	"net"
	"runtime"
	"strconv"
	"strings"
	"time"
)

/*
 * Defines a Service that can be published using UPnP or a specified set of denoted master nodes.
 */
type Service struct {
	AdType   string // service ad type
	Id       string // uuid
	Location string // adress/IP
	Service  string // service type
	Version  string // service version
	Server   string // server info
	MaxAge   int    // max caching age
}

/*
 * Access the host part of the Location field.
 */
func (s Service) Host() string {
	parts := strings.Split(s.Location, ":")
	return parts[0]
}

/*
 * Access the port part of the Location field.
 */
func (s Service) Port() int {
	parts := strings.Split(s.Location, ":")
	if len(parts) != 2 {
		panic("Invalid host:port location: " + s.Location)
	}
	i, err := strconv.Atoi(parts[1])
	if err != nil {
		return -1
	}
	return i
}

/*
 * Constructs a UPnP compatible USN field value.
 */
func (s Service) USN() string {
	return "uuid:" + s.Id + "::urn:winkube-org:service:" + s.Service + ":" + s.Version
}

/*
 * Constructs a UPnP compatible ST (service type) field value.
 */
func (s Service) ST() string {
	return "urn:winkube-org:service:" + s.Service + ":" + s.Version
}

/**
 * Extracts the service name of an URN of the form uuid:38a83898-fd17-4b84-a37d-2b4460d49e8f::urn:winkube-org:service:myService:1.
 * The examplke above results in myService.
 */
func ServiceFromUSN(usn string) string {
	parts := strings.Split(usn, "::")
	if len(parts) >= 2 {
		leftTrimmed := strings.TrimPrefix(parts[1], "urn:winkube-org:service:")
		return strings.TrimSuffix(leftTrimmed, ":1")
	}
	return usn
}

/**
 * Extracts the UUID of an URN of the form uuid:38a83898-fd17-4b84-a37d-2b4460d49e8f::urn:winkube-org:service:myService:1.
 * The examplke above results in 38a83898-fd17-4b84-a37d-2b4460d49e8f.
 */
func UUIDFromUSN(usn string) string {
	parts := strings.Split(usn, "::")
	if len(parts) >= 1 {
		return strings.TrimPrefix(parts[0], "uuid:")
	}
	return usn
}

/*
 * Interface for handling service announcements received via UPnP.
 */
type ServiceHandler interface {
	ServiceReceived(service Service)
}

/*
 * Multicast server instance that monitors and publishes services.
 */
type Multicast struct {
	adType           string
	localIp          string
	aliveInterval    int
	advertizers      map[string]*ssdp.Advertiser
	verbose          bool
	advertizing      bool
	monitor          *ssdp.Monitor
	providedServices func() []Service
	serviceHandlers  []ServiceHandler
}

/*
 * Creates a new Multicast server.
 */
func CreateMulticast(advertisementType string, providedServices func() []Service) *Multicast {
	multicastInstance := Multicast{
		adType:           advertisementType,
		providedServices: providedServices,
		aliveInterval:    30,
		verbose:          true,
		localIp:          GetInternalIP(),
		advertizers:      map[string]*ssdp.Advertiser{},
	}
	// start client
	multicastInstance.monitor = &ssdp.Monitor{
		Alive:  multicastInstance.onAlive,
		Bye:    multicastInstance.onBye,
		Search: multicastInstance.onSearch,
	}
	if err := multicastInstance.monitor.Start(); err != nil {
		log.Fatal(err)
	}
	return &multicastInstance
}

/*
 * Method called when a service is published on the UPnP bus.
 */
func (mc Multicast) onAlive(m *ssdp.AliveMessage) {
	if m.From.String() == mc.localIp {
		log.Debug("[SSDP] Ignoring my own alive ticket.")
	}
	s := Service{
		AdType:   m.Type,
		Id:       UUIDFromUSN(m.USN),
		Location: m.Location,
		Server:   m.Server,
		Service:  ServiceFromUSN(m.USN),
		MaxAge:   m.MaxAge(),
	}
	json, _ := json.MarshalIndent(s, "", "  ")
	log.Debugln("[SSDP] received ", string(json))
	for _, h := range mc.serviceHandlers {
		h.ServiceReceived(s)
	}
}

/*
 * Method called when a service is removed on the UPnP bus.
 */
func (mc Multicast) onBye(m *ssdp.ByeMessage) {
	s := Service{
		AdType:  m.Type,
		Id:      UUIDFromUSN(m.USN),
		Service: ServiceFromUSN(m.USN),
	}
	log.Info("Bye: From %s\n", s)
	if m.From.String() != mc.localIp {
		// TODO handle service exit...
	}
}

/*
 * Method called when a search request is published on the UPnP bus. Hereby the response is should be sent via
 * Unicast typically to port 1900 to the requesting location. This functions is inherently unsafe and
 * therefore will not be supported.
 */
func (mc Multicast) onSearch(m *ssdp.SearchMessage) {
	log.Info("Search request ignored: From=", m.From.String(), " Type=", m.Type)
	if mc.adType == m.Type {
		if mc.providedServices != nil {
			//services := mc.providedServices()
			if mc.verbose {
				log.Info("[SSDP] TODO Answering UPnP search request for %s", mc.adType)
			}
		}
	}
}

/*
 * Adds a listener for handling multicast service announcements.
 */
func (mc *Multicast) Listen(listener ServiceHandler) {
	mc.serviceHandlers = append(mc.serviceHandlers, listener)
}

/*
 * Start regularly publishing the services exposed by this instance.
 */
func (mc Multicast) StartAdvertizer() {
	if mc.advertizing {
		return
	}
	mc.advertizing = true
	if mc.providedServices != nil {
		services := mc.providedServices()
		if mc.verbose {
			log.Infoln("[SSDP] advertizing services: ", services)
		}
		for _, service := range services {
			usedAdvertisers := []string{}
			advertizer, err := ssdp.Advertise(service.ST(), service.USN(), service.Location, service.Server,
				service.MaxAge)
			if err != nil {
				log.Fatal(err)
			}
			mc.advertizers[service.USN()] = advertizer
			usedAdvertisers = append(usedAdvertisers, service.USN())
			if mc.verbose {
				advertizer.Alive()
				log.Debug("[SSDP] Advertized service: {\n" +
					"  AdType   : " + service.AdType + "\n" +
					"  Id       : " + service.Id + "\n" +
					"  Service  : " + service.Service + "\n" +
					"  Version  : " + service.Version + "\n" +
					"  Server   : " + service.Server + "\n" +
					"  Location : " + service.Location + "\n" +
					"  MaxAge   : " + strconv.Itoa(service.MaxAge) + "\n}")
			}
			time.Sleep(11 * time.Second)
		}
	}
	go mc.keepAliveLoop()
}

/*
 * Stops advertizing any services exposed by this instance.
 */
func (mc Multicast) StopAdvertisor() {
	mc.advertizing = false
}

func (mc Multicast) keepAliveLoop() {
	for mc.advertizing {
		if mc.providedServices != nil {
			services := mc.providedServices()
			if mc.verbose {
				log.Debug("[SSDP] sending keep alive for services ", services)
			}

			for _, service := range services {
				usedAdvertisers := []string{}
				advertizer := mc.advertizers[service.USN()]
				if &advertizer == nil {
					var err error
					advertizer, err = ssdp.Advertise(service.ST(), service.USN(), service.Location, service.Server,
						service.MaxAge)
					if err != nil {
						log.Fatal(err)
					}
					mc.advertizers[service.USN()] = advertizer
				}
				usedAdvertisers = append(usedAdvertisers, service.USN())
				if mc.verbose {
					advertizer.Alive()
					log.Debug("[SSDP] Advertized service: {\n" +
						"  AdType   : " + service.AdType + "\n" +
						"  Id       : " + service.Id + "\n" +
						"  Service  : " + service.Service + "\n" +
						"  Version  : " + service.Version + "\n" +
						"  Server   : " + service.Server + "\n" +
						"  Location : " + service.Location + "\n" +
						"  MaxAge   : " + strconv.Itoa(service.MaxAge) + "\n}")
				}
				if len(usedAdvertisers) < len(mc.advertizers) {
					log.Info("Some services have been removed, sending bye message...")
					unusedAdvertisers := map[string]ssdp.Advertiser{}
					for key, adv := range mc.advertizers {
						if !util.Exists(usedAdvertisers, key) {
							unusedAdvertisers[key] = *adv
						}
					}
					for key, adv := range unusedAdvertisers {
						log.Info("Removing service: %s\n", key)
						delete(mc.advertizers, key)
						adv.Bye()
						adv.Close()
					}
				}
				time.Sleep(11 * time.Second)
			}
		}
	}
}

/*
 * Test method to actively trigger a search request to the UPnP bus.
 */
func (mc Multicast) triggerSearch() {
	for {
		time.Sleep(5 * time.Second)
		servicesFound, _ := ssdp.Search(mc.adType, 8, mc.localIp)
		log.Debug("***** Services found: %s", servicesFound)
		time.Sleep(10 * time.Second)
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

/*
 * Calculates a runtme info as used in the UPnP SERVER field.
 */
func RuntimeInfo() string {
	return runtime.GOOS + "/" + runtime.GOARCH + " UPnP/1.0 WinKube/1.0"
}
