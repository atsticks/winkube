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
	"fmt"
	"github.com/koron/go-ssdp"
	log "github.com/sirupsen/logrus"
	util2 "github.com/winkube/util"
	"net"
	"strconv"
	"strings"
	"time"
)

// Defines a Service that can be published using UPnP or a specified set of denoted master nodes.
type Service struct {
	AdType   string `validate:"required"`
	Id       string `validate:"required"`
	Location string `validate:"required,ip"`
	Service  string `validate:"required"`
	Version  string `validate:"required"`
	Server   string
	MaxAge   int `validate:"required,min=0"`
}

// A ServiceRegistry publishes and reads services using multicast requests using the UPnP protocol.
// If no multicast is available/supported, expicit service catalogs can be configured instead of.
type ServiceRegistry interface {

	// Configures the registry as UPnP service using the given port (default is 1900). This
	// will restart any running service and clear all currently tracked services.
	Start(useMulticast bool, multicastPort int, catalogues []string)

	// This method explicitly stops the registry service. No collected service data will be removed, but no
	// further updates will happen.
	Stop()

	// Adds a provider that provides services to be published over UPnP/as remote controller endpoint
	AddServices(providerId string, services []Service)

	// Removes a provider that provided services to be published over UPnP/as remote controller endpoint
	RemoveServices(providerId string)

	// Registers a listener for received service events.
	Listen(listener *ServiceListener)

	// Unregisters a listener for received service events.
	Unlisten(listener *ServiceListener)
}

// Interface for providing services to be advertised
// it returns the provider id and the current list of services.
// To update the list of advertized services either recall this method,
// providing a different set of services or removing the provider by id completely.
type ServiceProvider func() (string, []Service)

// Interface for handling service announcements received via UPnP.
type ServiceListener interface {
	ServiceReceived(service Service)
}

// ServiceRegistry server instance that monitors and publishes services.
type serviceRegistry struct {
	adType          string  `validate:"required"`
	localIp         *net.IP `validate:"required, ip"`
	aliveInterval   int     `validate:"required,min=5"`
	advertizers     map[string]*ssdp.Advertiser
	verbose         bool
	advertizing     bool
	monitor         *ssdp.Monitor
	services        map[string][]Service
	serviceHandlers []*ServiceListener
}

func (this serviceRegistry) Start(useMulticast bool, multicastPort int, masterController []string) {
	if useMulticast {
		fmt.Println("Starting Service Registry: Multicast network...")
		this.startUPnP(multicastPort)
	} else {
		fmt.Println("Starting Service Registry: Unicast network...")
		this.startServiceCatalogue(masterController)
	}
	fmt.Println("Service registry started.")
}

// Creates a new ServiceRegistry server.
func CreateServiceRegistry(advertisementType string) *ServiceRegistry {
	var serviceRegistry serviceRegistry = serviceRegistry{
		adType:        advertisementType,
		aliveInterval: 30,
		verbose:       true,
		localIp:       GetDefaultIP(),
		advertizers:   map[string]*ssdp.Advertiser{},
		services:      map[string][]Service{},
	}
	// start client
	serviceRegistry.monitor = &ssdp.Monitor{
		Alive:  serviceRegistry.onAlive,
		Bye:    serviceRegistry.onBye,
		Search: serviceRegistry.onSearch,
	}
	if err := serviceRegistry.monitor.Start(); err != nil {
		log.Fatal(err)
	}
	var sm ServiceRegistry = &serviceRegistry
	return &sm
}

// Access the host part of the Location field.
func (this Service) Host() string {
	parts := strings.Split(this.Location, ":")
	return parts[0]
}

// Access the port part of the Location field.
func (this Service) Port() int {
	parts := strings.Split(this.Location, ":")
	if len(parts) != 2 {
		panic("Invalid host:port location: " + this.Location)
	}
	i, err := strconv.Atoi(parts[1])
	if err != nil {
		return -1
	}
	return i
}

// Constructs a UPnP compatible USN field value.
func (this Service) USN() string {
	return "uuid:" + this.Id + "::urn:winkube-org:service:" + this.Service + ":" + this.Version
}

// Constructs a UPnP compatible ST (service type) field value.
func (this Service) ST() string {
	return "urn:winkube-org:service:" + this.Service + ":" + this.Version
}

// Extracts the service name of an URN of the form uuid:38a83898-fd17-4b84-a37d-2b4460d49e8f::urn:winkube-org:service:myService:1.
// The examplke above results in myService.
func ServiceFromUSN(usn string) string {
	parts := strings.Split(usn, "::")
	if len(parts) >= 2 {
		leftTrimmed := strings.TrimPrefix(parts[1], "urn:winkube-org:service:")
		return strings.TrimSuffix(leftTrimmed, ":1")
	}
	return usn
}

// Extracts the UUID of an URN of the form uuid:38a83898-fd17-4b84-a37d-2b4460d49e8f::urn:winkube-org:service:myService:1.
// The examplke above results in 38a83898-fd17-4b84-a37d-2b4460d49e8f.
func UUIDFromUSN(usn string) string {
	parts := strings.Split(usn, "::")
	if len(parts) >= 1 {
		return strings.TrimPrefix(parts[0], "uuid:")
	}
	return usn
}

// Method called when a service is published on the UPnP bus.
func (this *serviceRegistry) onAlive(m *ssdp.AliveMessage) {
	if m.From.String() == this.localIp.String() {
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
	for _, h := range this.serviceHandlers {
		(*h).ServiceReceived(s)
	}
}

// Method called when a service is removed on the UPnP bus.
func (this *serviceRegistry) onBye(m *ssdp.ByeMessage) {
	s := Service{
		AdType:  m.Type,
		Id:      UUIDFromUSN(m.USN),
		Service: ServiceFromUSN(m.USN),
	}
	log.Info(fmt.Sprintf("Bye: From %v\n", s))
	if m.From.String() != this.localIp.String() {
		// TODO handle service exit...
	}
}

// Method called when a search request is published on the UPnP bus. Hereby the response is should be sent via
// Unicast typically to port 1900 to the requesting location. This functions is inherently unsafe and
//therefore will not be supported.
func (this *serviceRegistry) onSearch(m *ssdp.SearchMessage) {
	log.Info("Search request ignored: From=", m.From.String(), " Type=", m.Type)
	if this.adType == m.Type {
		if len(this.services) > 0 {
			//services := mc.serviceProvider()
			if this.verbose {
				log.Info(fmt.Sprintf("[SSDP] TODO Answering UPnP search request for %v", this.adType))
			}
		}
	}
}

// Adds a listener for handling multicast service announcements.
func (this *serviceRegistry) AddServices(providerId string, services []Service) {
	this.services[providerId] = services
}

// Removes a listener for handling multicast service announcements.
func (this *serviceRegistry) RemoveServices(providerId string) {
	this.services[providerId] = nil
}

// Adds a listener for handling multicast service announcements.
func (this *serviceRegistry) Listen(listener *ServiceListener) {
	this.serviceHandlers = append(this.serviceHandlers, listener)
}

// Removes a listener for handling multicast service announcements.
func (this *serviceRegistry) Unlisten(listener *ServiceListener) {
	index := util2.IndexOf(this.serviceHandlers, listener)
	if index >= 0 {
		this.serviceHandlers = append(this.serviceHandlers[:index], this.serviceHandlers[index:]...)
	}
}

// start regularly publishing the services exposed by this instance.
func (this *serviceRegistry) startServiceCatalogue(catalogs []string) {
	panic("Not implemented: StartServiceCatalogue")
}

// start regularly publishing the services exposed by this instance.
func (this *serviceRegistry) startUPnP(port int) {
	if port == 0 {
		port = 1900
	}
	if this.advertizing {
		this.advertizing = false
	}
	time.Sleep(20 * time.Second)
	go this.keepAliveLoop()
}

/*
 * Stops advertizing any services exposed by this instance.
 */
func (this *serviceRegistry) Stop() {
	this.advertizing = false
}

func (this *serviceRegistry) keepAliveLoop() {
	this.advertizing = true
	for this.advertizing {
		for providerId, services := range this.services {
			log.Debug(fmt.Sprintf("[SSDP] Advertizing services for provider: %v ...", providerId))
			for _, service := range services {
				usedAdvertisers := []string{}
				advertizer, err := ssdp.Advertise(service.ST(), service.USN(), service.Location, service.Server,
					service.MaxAge)
				if err != nil {
					log.Fatal(err)
				}
				this.advertizers[service.USN()] = advertizer
				usedAdvertisers = append(usedAdvertisers, service.USN())
				if this.verbose {
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
				if len(usedAdvertisers) < len(this.advertizers) {
					log.Info("Some services have been removed, sending bye message...")
					unusedAdvertisers := map[string]ssdp.Advertiser{}
					for key, adv := range this.advertizers {
						if !util2.Exists(usedAdvertisers, key) {
							unusedAdvertisers[key] = *adv
						}
					}
					for key, adv := range unusedAdvertisers {
						log.Info(fmt.Sprintf("Removing service: %s\n", key))
						delete(this.advertizers, key)
						adv.Bye()
						adv.Close()
					}
				}
				time.Sleep(15 * time.Second)
			}
		}
	}
}

/*
 * Test method to actively trigger a search request to the UPnP bus.
 */
func (this *serviceRegistry) triggerSearch() {
	for {
		time.Sleep(5 * time.Second)
		servicesFound, _ := ssdp.Search(this.adType, 8, this.localIp.String())
		log.Debug(fmt.Sprintf("***** Services found: %v", servicesFound))
		time.Sleep(10 * time.Second)
	}

}

// Evaluates all adresses and takes the first non loopback IP presenbt.
func GetDefaultIP() *net.IP {
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
	return &ip
}

func GetDefaultInterface() *net.Interface {
	ifaces, _ := net.Interfaces() //here your interface

	for _, iface := range ifaces {
		adresses, _ := iface.Addrs()
		for _, address := range adresses {
			switch v := address.(type) {
			case *net.IPNet:
				if !v.IP.IsLoopback() {
					if v.IP.To4() != nil { //Verify if IP is IPV4
						return &iface
					}
				}
			}
		}
	}
	return nil
}

/*
 * Calculates a runtme info as used in the UPnP SERVER field.
 */
func runtimeInfo() string {
	return util2.RuntimeInfo() + " UPnP/1.0 WinKube/1.0"
}
