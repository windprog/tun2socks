package tunbased

import (
	"github.com/google/netstack/tcpip"
	"github.com/google/netstack/tcpip/buffer"
	"github.com/google/netstack/tcpip/header"
	"github.com/google/netstack/tcpip/stack"
	"github.com/songgao/water"
	"runtime"
	"sync"
)

type endpoint struct {
	// ifce is tun device of songgao/water
	writer NonBlockingRWInterface

	// mtu (maximum transmission unit) is the maximum size of a packet.
	mtu uint32

	// hdrSize specifies the link-layer header size. If set to 0, no header
	// is added/removed; otherwise an ethernet header is used.
	hdrSize int

	// addr is the address of the *endpoint.
	addr tcpip.LinkAddress

	// caps holds the *endpoint capabilities.
	caps stack.LinkEndpointCapabilities

	// closed is a function to be called when the FD's peer (if any) closes
	// its end of the communication pipe.
	closed func(*tcpip.Error)

	dispatcher stack.NetworkDispatcher

	// gsoMaxSize is the maximum GSO packet size. It is zero if GSO is
	// disabled.
	gsoMaxSize uint32

	// wg keeps track of running goroutines.
	wg sync.WaitGroup
}

// Options specify the details about the fd-based endpoint to be created.
type Options struct {
	// ifce is tun device of songgao/water
	Ifce *water.Interface

	// MTU is the mtu to use for this endpoint.
	MTU uint32

	// EthernetHeader if true, indicates that the *endpoint should read/write
	// ethernet frames instead of IP packets.
	EthernetHeader bool

	// ClosedFunc is a function to be called when an endpoint's peer (if
	// any) closes its end of the communication pipe.
	ClosedFunc func(*tcpip.Error)

	// Address is the link address for this endpoint. Only used if
	// EthernetHeader is true.
	Address tcpip.LinkAddress

	// SaveRestore if true, indicates that this NIC capability set should
	// include CapabilitySaveRestore
	SaveRestore bool

	// DisconnectOk if true, indicates that this NIC capability set should
	// include CapabilityDisconnectOk.
	DisconnectOk bool

	// TXChecksumOffload if true, indicates that this endpoints capability
	// set should include CapabilityTXChecksumOffload.
	TXChecksumOffload bool

	// RXChecksumOffload if true, indicates that this endpoints capability
	// set should include CapabilityRXChecksumOffload.
	RXChecksumOffload bool
}

func New(opts *Options) (stack.LinkEndpoint, error) {
	caps := stack.LinkEndpointCapabilities(0)
	if opts.RXChecksumOffload {
		caps |= stack.CapabilityRXChecksumOffload
	}

	if opts.TXChecksumOffload {
		caps |= stack.CapabilityTXChecksumOffload
	}

	hdrSize := 0
	if opts.EthernetHeader {
		hdrSize = header.EthernetMinimumSize
		caps |= stack.CapabilityResolutionRequired
	}

	if opts.SaveRestore {
		caps |= stack.CapabilitySaveRestore
	}

	if opts.DisconnectOk {
		caps |= stack.CapabilityDisconnectOk
	}

	e := &endpoint{
		mtu:     opts.MTU,
		caps:    caps,
		closed:  opts.ClosedFunc,
		addr:    opts.Address,
		hdrSize: hdrSize,
	}

	var writer NonBlockingRWInterface
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		writer = NewUnixWriter(e, opts.Ifce)
		// for normal debug
		//writer = NewWriterNormal(e, opts.Ifce)
	} else {
		writer = NewWriterNormal(e, opts.Ifce)
	}
	e.writer = writer

	return e, nil
}

// MTU implements stack.LinkEndpoint.MTU. It returns the value initialized
// during construction.
func (e *endpoint) MTU() uint32 {
	return e.mtu
}

// Capabilities implements stack.LinkEndpoint.Capabilities.
func (e *endpoint) Capabilities() stack.LinkEndpointCapabilities {
	return e.caps
}

// MaxHeaderLength returns the maximum size of the link-layer header.
func (e *endpoint) MaxHeaderLength() uint16 {
	return uint16(e.hdrSize)
}

// LinkAddress returns the link address of this endpoint.
func (e *endpoint) LinkAddress() tcpip.LinkAddress {
	return e.addr
}

// WritePacket writes outbound packets to the file descriptor. If it is not
// currently writable, the packet is dropped.
func (e *endpoint) WritePacket(r *stack.Route, gso *stack.GSO, protocol tcpip.NetworkProtocolNumber, pkt tcpip.PacketBuffer) *tcpip.Error {
	if e.hdrSize > 0 {
		// Add ethernet header if needed.
		eth := header.Ethernet(pkt.Header.Prepend(header.EthernetMinimumSize))
		pkt.LinkHeader = buffer.View(eth)
		ethHdr := &header.EthernetFields{
			DstAddr: r.RemoteLinkAddress,
			SrcAddr: e.addr,
			Type:    protocol,
		}

		// Preserve the src address if it's set in the route.
		if r.LocalLinkAddress != "" {
			ethHdr.SrcAddr = r.LocalLinkAddress
		} else {
			ethHdr.SrcAddr = e.addr
		}
		eth.Encode(ethHdr)
	}

	if pkt.Data.Size() == 0 {
		return e.writer.NonBlockingWrite(pkt.Header.View())
	}

	return e.writer.NonBlockingWrite3(pkt.Header.View(), pkt.Data.ToView(), nil)
}

func (e *endpoint) WritePackets(r *stack.Route, gso *stack.GSO, hdrs []stack.PacketDescriptor, payload buffer.VectorisedView, protocol tcpip.NetworkProtocolNumber) (int, *tcpip.Error) {
	panic("Not Implement")
}

func (e *endpoint) WriteRawPacket(vv buffer.VectorisedView) *tcpip.Error {
	return e.writer.NonBlockingWrite(vv.ToView())
}

// Attach launches the goroutine that reads packets from the file descriptor and
// dispatches them via the provided dispatcher.
func (e *endpoint) Attach(dispatcher stack.NetworkDispatcher) {
	e.dispatcher = dispatcher
	// Link endpoints are not savable. When transportation endpoints are
	// saved, they stop sending outgoing packets and all incoming packets
	// are rejected.
	e.wg.Add(1)
	go func() {
		e.writer.DispatchInboundLoop()
		e.wg.Done()
	}()
}

// IsAttached implements stack.LinkEndpoint.IsAttached.
func (e *endpoint) IsAttached() bool {
	return e.dispatcher != nil
}

// Wait implements stack.LinkEndpoint.Wait. It waits for the endpoint to stop
// reading from its FD.
func (e *endpoint) Wait() {
	e.wg.Wait()
}

var _ stack.LinkEndpoint = &endpoint{}
