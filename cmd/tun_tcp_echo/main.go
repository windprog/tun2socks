package main

import (
	"fmt"
	"github.com/google/netstack/tcpip"
	"github.com/google/netstack/tcpip/network/ipv4"
	"github.com/google/netstack/tcpip/network/ipv6"
	"github.com/google/netstack/tcpip/stack"
	"github.com/google/netstack/tcpip/transport/tcp"
	"github.com/google/netstack/waiter"
	"github.com/songgao/water"
	"log"
	"math/rand"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
	"tun2socks/tunbased"
)

// echo service
func echo(wq *waiter.Queue, ep tcpip.Endpoint) {
	defer ep.Close()

	// Create wait queue entry that notifies a channel.
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)

	wq.EventRegister(&waitEntry, waiter.EventIn)
	defer wq.EventUnregister(&waitEntry)

	for {
		v, _, err := ep.Read(nil)
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				<-notifyCh
				continue
			}

			return
		}

		ep.Write(tcpip.SlicePayload(v), tcpip.WriteOptions{})
	}
}

func getFieldItem(target reflect.Value, fieldName string) reflect.Value {
	names := strings.Split(fieldName, ".")
	cntName := names[0]
	for i := 0; i < target.NumField(); i++ {
		name := target.Type().Field(i).Name
		value := target.Field(i)
		if name != cntName {
			continue
		}
		if len(names) >= 2 {
			return getFieldItem(value, strings.Join(names[1:], ","))
		}
		return value
	}
	panic(fmt.Sprintf("%v field:%s not found", target, fieldName))
}

// go build -o tun_tcp_echo
// sudo ./tun_tcp_echo utun2 10.0.0.2 8090
// telnet 10.0.0.2 8090
func main() {
	if len(os.Args) != 4 {
		log.Fatal("Usage: ", os.Args[0], " <tun-device> <local-address> <local-port>")
	}

	// tunName := os.Args[1]
	addrName := os.Args[2]
	portName := os.Args[3]

	rand.Seed(time.Now().UnixNano())

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Parse the IP address. Support both ipv4 and ipv6.
	parsedAddr := net.ParseIP(addrName)
	if parsedAddr == nil {
		log.Fatalf("Bad IP address: %v", addrName)
	}

	var addr tcpip.Address
	var proto tcpip.NetworkProtocolNumber
	if parsedAddr.To4() != nil {
		addr = tcpip.Address(parsedAddr.To4())
		proto = ipv4.ProtocolNumber
	} else if parsedAddr.To16() != nil {
		addr = tcpip.Address(parsedAddr.To16())
		proto = ipv6.ProtocolNumber
	} else {
		log.Fatalf("Unknown IP type: %v", addrName)
	}

	localPort, err := strconv.Atoi(portName)
	if err != nil {
		log.Fatalf("Unable to convert port %v: %v", portName, err)
	}

	// Create the stack with ip and tcp protocols, then add a tun-based
	// NIC and address.
	s := stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocol{ipv4.NewProtocol(), ipv6.NewProtocol()},
		TransportProtocols: []stack.TransportProtocol{tcp.NewProtocol()},
	})

	var mtu uint32 = 1500

	// Parse the mac address.
	maddr, err := net.ParseMAC("aa:00:01:01:01:01")
	if err != nil {
		log.Fatalf("Bad MAC address: aa:00:01:01:01:01")
	}

	ifce, err := water.New(tunbased.GetCfg())
	if err != nil {
		log.Fatalf("tun/tap can't init. err:%v", err)
	}
	tunbased.Ifconfig(ifce.Name(), "10.0.0.2/24", mtu)

	linkEP, err := tunbased.New(&tunbased.Options{
		Ifce:           ifce,
		MTU:            mtu,
		EthernetHeader: false,
		Address:        tcpip.LinkAddress(maddr),
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := s.CreateNIC(1, linkEP); err != nil {
		log.Fatal(err)
	}

	if err := s.AddAddress(1, proto, addr); err != nil {
		log.Fatal(err)
	}

	//if err := s.AddAddress(1, arp.ProtocolNumber, arp.ProtocolAddress); err != nil {
	//	log.Fatal(err)
	//}

	subnet, err := tcpip.NewSubnet(
		tcpip.Address(strings.Repeat("\x00", len(addr))),
		tcpip.AddressMask(strings.Repeat("\x00", len(addr))),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Add default route.
	s.SetRouteTable([]tcpip.Route{
		{
			Destination: subnet,
			NIC:         1,
		},
	})

	// Create TCP endpoint, bind it, then start listening.
	var wq waiter.Queue
	ep, e := s.NewEndpoint(tcp.ProtocolNumber, proto, &wq)
	if err != nil {
		log.Fatal(e)
	}

	defer ep.Close()

	log.Printf("listening:%d", localPort)

	if err := ep.Bind(tcpip.FullAddress{0, "", uint16(localPort)}); err != nil {
		log.Fatal("Bind failed: ", err)
	}

	if err := ep.Listen(10); err != nil {
		log.Fatal("Listen failed: ", err)
	}

	// Wait for connections to appear.
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)
	wq.EventRegister(&waitEntry, waiter.EventIn)
	defer wq.EventUnregister(&waitEntry)

	for {
		n, wq, err := ep.Accept()
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				<-notifyCh
				continue
			}

			log.Fatal("Accept() failed:", err)
		}

		go echo(wq, n)
	}
}
