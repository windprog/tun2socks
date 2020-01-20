// +build linux

package tunbased

import (
	"github.com/google/netstack/tcpip"
	"syscall"
	"unsafe"
)

// NonBlockingWrite writes the given buffer to a file descriptor. It fails if
// partial data is written.
func (w *WaterRWUnixReadv) NonBlockingWrite(buf []byte) *tcpip.Error {
	fd := int(GetWaterIfF(w.ifce).Fd())
	var ptr unsafe.Pointer
	if len(buf) > 0 {
		ptr = unsafe.Pointer(&buf[0])
	} else {
		return nil
	}

	_, _, e := syscall.RawSyscall(syscall.SYS_WRITE, uintptr(fd), uintptr(ptr), uintptr(len(buf)))
	if e != 0 {
		return TranslateErrno(e)
	}

	return nil
}

// NonBlockingWrite3 writes up to three byte slices to a file descriptor in a
// single syscall. It fails if partial data is written.
func (w *WaterRWUnixReadv) NonBlockingWrite3(b1, b2, b3 []byte) *tcpip.Error {

	fd := int(GetWaterIfF(w.ifce).Fd())
	// If the is no second buffer, issue a regular write.
	if len(b2) == 0 {
		return w.NonBlockingWrite(b1)
	}

	// We have two buffers. Build the iovec that represents them and issue
	// a writev syscall.
	iovec := [3]syscall.Iovec{
		{
			Base: &b1[0],
			Len:  uint64(len(b1)),
		},
		{
			Base: &b2[0],
			Len:  uint64(len(b2)),
		},
	}
	iovecLen := uintptr(2)

	if len(b3) > 0 {
		iovecLen++
		iovec[2].Base = &b3[0]
		iovec[2].Len = uint64(len(b3))
	}

	_, _, e := syscall.RawSyscall(syscall.SYS_WRITEV, uintptr(fd), uintptr(unsafe.Pointer(&iovec[0])), iovecLen)
	if e != 0 {
		return TranslateErrno(e)
	}

	return nil
}