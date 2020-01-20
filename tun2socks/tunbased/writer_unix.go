// +build Linux darwin

package tunbased

import (
	"fmt"
	"github.com/google/netstack/tcpip"
	"github.com/google/netstack/tcpip/buffer"
	"github.com/google/netstack/tcpip/header"
	"github.com/songgao/water"
	"syscall"
	"tun2socks/third_party/tunbased/rawfile_block"
)

type WaterRWUnixReadv struct {
	// 控制 tun 设备
	ifce *water.Interface
	// 读取配置
	e *endpoint

	// views are the actual buffers that hold the packet contents.
	views []buffer.View

	// iovecs are initialized with base pointers/len of the corresponding
	// entries in the views defined above, except when GSO is enabled then
	// the first iovec points to a buffer for the vnet header which is
	// stripped before the views are passed up the stack for further
	// processing.
	iovecs []syscall.Iovec
}

func NewUnixWriter(e *endpoint, ifce *water.Interface) *WaterRWUnixReadv {
	w := &WaterRWUnixReadv{
		ifce: ifce,
		e:    e,
	}
	w.views = make([]buffer.View, len(BufConfig))
	iovLen := len(BufConfig)
	w.iovecs = make([]syscall.Iovec, iovLen)
	return w
}

// dispatchLoop reads packets from the file descriptor in a loop and dispatches
// them to the network stack.
func (w *WaterRWUnixReadv) DispatchInboundLoop() *tcpip.Error {
	for {
		cont, err := w.dispatch()
		if err != nil || !cont {
			if w.e.closed != nil {
				w.e.closed(err)
			}
			return err
		}
	}
}

var _ NonBlockingRWInterface = &WaterRWUnixReadv{}

// next from https://github.com/google/netstack/blob/master/tcpip/link/fdbased/packet_dispatchers.go

// BufConfig defines the shape of the vectorised view used to read packets from the NIC.
var BufConfig = []int{128, 256, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768}

func (w *WaterRWUnixReadv) allocateViews(bufConfig []int) {

	for i := 0; i < len(bufConfig); i++ {
		if w.views[i] != nil {
			break
		}
		b := buffer.NewView(bufConfig[i])
		w.views[i] = b
		w.iovecs[i] = syscall.Iovec{
			Base: &b[0],
			Len:  uint64(len(b)),
		}
	}
}

func (w *WaterRWUnixReadv) capViews(n int, buffers []int) int {
	c := 0
	for i, s := range buffers {
		c += s
		if c >= n {
			w.views[i].CapLength(s - (c - n))
			return i + 1
		}
	}
	return len(buffers)
}

// dispatch reads one packet from the file descriptor and dispatches it.
func (w *WaterRWUnixReadv) dispatch() (bool, *tcpip.Error) {
	// darwin version
	w.allocateViews(BufConfig)
	fd := int(GetWaterIfF(w.ifce).Fd())

	n, err := rawfile_block.BlockingReadv(fd, w.iovecs)
	if err != nil {
		return false, err
	}
	if n <= w.e.hdrSize {
		return false, nil
	}
	fmt.Printf("in pack:%d\n", n)

	var (
		p             tcpip.NetworkProtocolNumber
		remote, local tcpip.LinkAddress
		eth           header.Ethernet
	)
	if w.e.hdrSize > 0 {
		eth = header.Ethernet(w.views[0][4 : header.EthernetMinimumSize+4])
		p = eth.Type()
		remote = eth.SourceAddress()
		local = eth.DestinationAddress()
	} else {
		// We don't get any indication of what the packet is, so try to guess
		// if it's an IPv4 or IPv6 packet.
		switch header.IPVersion(w.views[0][4:]) {
		case header.IPv4Version:
			p = header.IPv4ProtocolNumber
		case header.IPv6Version:
			p = header.IPv6ProtocolNumber
		default:
			return true, nil
		}
	}

	used := w.capViews(n, BufConfig)
	buffData := []buffer.View(nil)
	buffData = append(buffData, w.views[0][4:])
	if used >= 2 {
		buffData = append(buffData, w.views[1:used]...)
	}
	pkt := tcpip.PacketBuffer{
		Data:       buffer.NewVectorisedView(n, buffData),
		LinkHeader: buffer.View(eth),
	}
	pkt.Data.TrimFront(w.e.hdrSize)

	w.e.dispatcher.DeliverNetworkPacket(w.e, remote, local, p, pkt)

	// Prepare e.views for another packet: release used views.
	for i := 0; i < used; i++ {
		w.views[i] = nil
	}

	return true, nil
}
