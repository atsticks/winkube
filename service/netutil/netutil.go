package netutil

import (
	"log"
	"net"
	"sync"
	"time"
)

type MessageHandler interface{
	MsgReceived(src *net.UDPAddr, message string)
}

type Multicast interface{
	Start(out func() string, messageHandler MessageHandler)
}

type MulticastInstance struct {
	srvAddr string
	maxDatagramSize int
}

var (
	instance MulticastInstance
	once sync.Once
)

func GetMulticast() Multicast {
	once.Do(func() {
		instance = MulticastInstance{srvAddr: "239.0.0.0:9999", maxDatagramSize: 8192}
	})
	return instance
}

func (mc *MulticastInstance) publish(out func() string) {
	for ; ;{
		addr, err := net.ResolveUDPAddr("udp", mc.srvAddr)
		if err != nil {
			log.Fatal(err)
			return
		}
		c, err := net.DialUDP("udp", nil, addr)
		message := out()
		log.Println("Sending Multicast: " + message)
		c.Write([]byte(message))
		time.Sleep(5 * time.Second)
	}
}

func (mc *MulticastInstance) serveMulticastUDP(address string, listener MessageHandler) {
	resolvedAddress, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		log.Fatal(err)
	}
	l, err := net.ListenMulticastUDP("udp", nil, resolvedAddress)
	if err != nil {
		log.Fatal("ReadFromUDP failed:", err)
		return
	}
	l.SetReadBuffer(mc.maxDatagramSize)
	for {
		b := make([]byte, mc.maxDatagramSize)
		n, src, err := l.ReadFromUDP(b)
		if err != nil {
			log.Fatal("ReadFromUDP failed:", err)
			return
		}
		if(n>0) {
			var message string = string(b)
			log.Println("Multicast received: " + message)
			listener.MsgReceived(src, message)
		}
	}
}

func (mc MulticastInstance) Start(out func() string, messageHandler MessageHandler) {
	go mc.publish(out)
	mc.serveMulticastUDP(mc.srvAddr, messageHandler)
}

// Evakuates all adresses and takes the first non loopback IP presenbt.
func GetInternalIP() string {
	addresses, _ := net.InterfaceAddrs() //here your interface
	var ip net.IP
	for _, addr := range addresses {
		switch v := addr.(type) {
		case *net.IPNet:
			if !v.IP.IsLoopback() {
				if v.IP.To4() != nil && ip==nil{//Verify if IP is IPV4
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


