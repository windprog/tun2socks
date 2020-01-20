// +build darwin

package tunbased

import (
	"github.com/google/netstack/tcpip"
	"syscall"
	"tun2socks/third_party/tunbased/rawfile_block"
	"unsafe"
)

func (w *WaterRWUnixReadv) NonBlockingWrite(buf []byte) *tcpip.Error {
	// darwin version
	fd := int(GetWaterIfF(w.ifce).Fd())
	if len(buf) == 0 {
		return nil
	}

	// Determine the IP Family for the NULL L2 Header
	// 写成静态内存
	header := make([]byte, 4)
	ipVer := buf[0] >> 4
	if ipVer == 4 {
		header[3] = syscall.AF_INET
	} else if ipVer == 6 {
		header[3] = syscall.AF_INET6
	} else {
		return MakeTcpipError("Unable to determine IP version from packet", false)
	}

	// We have two buffers. Build the iovec that represents them and issue
	// a writev syscall.
	iovec := [2]syscall.Iovec{
		{
			Base: &header[0],
			Len:  uint64(4),
		},
		{
			Base: &buf[0],
			Len:  uint64(len(buf)),
		},
	}
	iovecLen := uintptr(2)

	_, _, e := syscall.RawSyscall(syscall.SYS_WRITEV, uintptr(fd), uintptr(unsafe.Pointer(&iovec[0])), iovecLen)
	if e != 0 {
		return rawfile_block.TranslateErrno(e)
	}

	return nil
}

// NonBlockingWrite3 writes up to three byte slices to a file descriptor in a
// single syscall. It fails if partial data is written.
func (w *WaterRWUnixReadv) NonBlockingWrite3(b1, b2, b3 []byte) *tcpip.Error {
	// darwin version
	fd := int(GetWaterIfF(w.ifce).Fd())
	// If the is no second buffer, issue a regular write.
	if len(b2) == 0 {
		return w.NonBlockingWrite(b1)
	}

	// Determine the IP Family for the NULL L2 Header
	header := make([]byte, 4)
	ipVer := b1[0] >> 4
	if ipVer == 4 {
		header[3] = syscall.AF_INET
	} else if ipVer == 6 {
		header[3] = syscall.AF_INET6
	} else {
		return MakeTcpipError("Unable to determine IP version from packet", false)
	}

	// We have two buffers. Build the iovec that represents them and issue
	// a writev syscall.
	iovec := [4]syscall.Iovec{
		{
			Base: &header[0],
			Len:  uint64(4),
		},
		{
			Base: &b1[0],
			Len:  uint64(len(b1)),
		},
		{
			Base: &b2[0],
			Len:  uint64(len(b2)),
		},
	}
	iovecLen := uintptr(3)

	if len(b3) > 0 {
		iovecLen++
		iovec[3].Base = &b3[0]
		iovec[3].Len = uint64(len(b3))
	}

	_, _, e := syscall.RawSyscall(syscall.SYS_WRITEV, uintptr(fd), uintptr(unsafe.Pointer(&iovec[0])), iovecLen)
	if e != 0 {
		return rawfile_block.TranslateErrno(e)
	}

	return nil
}