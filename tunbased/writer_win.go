package tunbased

import (
	"encoding/hex"
	"fmt"
	"github.com/google/netstack/tcpip"
	"github.com/google/netstack/tcpip/buffer"
	"github.com/google/netstack/tcpip/header"
	"github.com/songgao/water"
	"log"
	"sync"
)

type WaterRWNormal struct {
	ifce *water.Interface
	e    *endpoint

	rMu  sync.Mutex
	rBuf []byte

	wMu  sync.Mutex
	wBuf []byte
}

func NewWriterNormal(e *endpoint, ifce *water.Interface) *WaterRWNormal {
	return &WaterRWNormal{
		ifce: ifce,
		e:    e,
	}
}

func (w *WaterRWNormal) writeCache() *tcpip.Error {
	_, err := w.ifce.Write(w.wBuf)
	if err != nil {
		//log.Println(err)
		return MakeTcpipError(err.Error(), false)
	}
	return nil
}

func (w *WaterRWNormal) NonBlockingWrite(buf []byte) *tcpip.Error {
	w.wMu.Lock()
	defer w.wMu.Unlock()

	if cap(w.wBuf) < len(buf) {
		w.wBuf = make([]byte, len(buf))
	}
	w.wBuf = w.wBuf[:len(buf)]

	copy(w.wBuf, buf)

	return w.writeCache()
}

func (w *WaterRWNormal) NonBlockingWrite3(b1, b2, b3 []byte) *tcpip.Error {
	w.wMu.Lock()
	defer w.wMu.Unlock()

	total := len(b1)
	if b2 != nil {
		total += len(b2)
	}
	if b3 != nil {
		total += len(b3)
	}
	if cap(w.wBuf) < total {
		w.wBuf = make([]byte, total)
	}

	w.wBuf = w.wBuf[:total]
	copy(w.wBuf[0:len(b1)], b1)
	ready := len(b1)

	if b2 != nil {
		copy(w.wBuf[ready:ready+len(b2)], b2)
		ready += len(b2)
	}

	if b3 != nil {
		copy(w.wBuf[ready:ready+len(b3)], b3)
		ready += len(b3)
	}

	return w.writeCache()
}

// dispatch reads one packet from the file descriptor and dispatches it.
func (w *WaterRWNormal) dispatch() (bool, *tcpip.Error) {
	packet := make([]byte, w.e.MTU())
	n, err := w.ifce.Read(packet)
	if err != nil {
		return false, MakeTcpipError(fmt.Sprintf("Read from tun failed:%v", err), false)
	}
	fmt.Printf("in pack:%d\n", n)
	log.Printf("info:\n%s\n", hex.Dump(packet[:n]))

	if n <= w.e.hdrSize {
		return false, nil
	}

	var (
		p             tcpip.NetworkProtocolNumber
		remote, local tcpip.LinkAddress
		eth           header.Ethernet
	)
	if w.e.hdrSize > 0 {
		eth = header.Ethernet(packet)
		p = eth.Type()
		remote = eth.SourceAddress()
		local = eth.DestinationAddress()
	} else {
		// We don't get any indication of what the packet is, so try to guess
		// if it's an IPv4 or IPv6 packet.
		switch header.IPVersion(packet) {
		case header.IPv4Version:
			p = header.IPv4ProtocolNumber
		case header.IPv6Version:
			p = header.IPv6ProtocolNumber
		default:
			return true, nil
		}
	}

	pkt := tcpip.PacketBuffer{
		Data:       buffer.NewVectorisedView(n, append([]buffer.View(nil), packet)),
		LinkHeader: buffer.View(eth),
	}
	pkt.Data.TrimFront(w.e.hdrSize)

	w.e.dispatcher.DeliverNetworkPacket(w.e, remote, local, p, pkt)
	// Prepare e.views for another packet: release used views.
	return true, nil
}

func (w *WaterRWNormal) DispatchInboundLoop() *tcpip.Error {
	// TODO 这里要模仿: packet_dispatchers.go 写循环收取数据
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

var _ NonBlockingRWInterface = &WaterRWNormal{}
