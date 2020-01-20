package tunbased

import "github.com/google/netstack/tcpip"

type NonBlockingRWInterface interface {
	NonBlockingWrite(buf []byte) *tcpip.Error
	NonBlockingWrite3(b1, b2, b3 []byte) *tcpip.Error
	DispatchInboundLoop() *tcpip.Error
}
