// +build Linux darwin
// from https://github.com/google/netstack/blob/master/tcpip/link/rawfile/rawfile_unsafe.go

package rawfile_block

import (
	"github.com/google/netstack/tcpip"
	"syscall"
	"unsafe"
)

// PollEvent represents the pollfd structure passed to a poll() system call.
type PollEvent struct {
	FD      int32
	Events  int16
	Revents int16
}

// BlockingReadv reads from a file descriptor that is set up as non-blocking and
// stores the data in a list of iovecs buffers. If no data is available, it will
// block in a poll() syscall until the file descriptor becomes readable.
func BlockingReadv(fd int, iovecs []syscall.Iovec) (int, *tcpip.Error) {

	for {
		n, _, e := syscall.RawSyscall(syscall.SYS_READV, uintptr(fd), uintptr(unsafe.Pointer(&iovecs[0])), uintptr(len(iovecs)))
		if e == 0 {
			return int(n), nil
		}

		event := PollEvent{
			FD:     int32(fd),
			Events: 1, // POLLIN
		}

		_, e = BlockingPoll(&event, 1, nil)
		if e != 0 && e != syscall.EINTR {
			return 0, TranslateErrno(e)
		}
	}
}
