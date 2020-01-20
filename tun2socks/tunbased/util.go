package tunbased

import (
	"fmt"
	"github.com/google/netstack/tcpip"
	"github.com/songgao/water"
	"log"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"strings"
	"unsafe"
)

// ExecCommand ...
func ExecCommand(name, sargs string) error {
	args := strings.Split(sargs, " ")
	cmd := exec.Command(name, args...)
	log.Printf("[command] %s %s", name, sargs)
	return cmd.Run()
}

func Ipv4MaskString(m []byte) string {
	if len(m) != 4 {
		log.Println("Len must be 4 bytes")
	}

	return fmt.Sprintf("%d.%d.%d.%d", m[0], m[1], m[2], m[3])
}

func SetUnexportedField(v reflect.Value) reflect.Value {
	if !v.CanSet() {
		v = reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
	}
	return v
}

func MakeTcpipError(msg string, ignoreStats bool) *tcpip.Error {
	ret := &tcpip.Error{}
	value := reflect.ValueOf(ret).Elem()
	SetUnexportedField(value.FieldByName("msg")).SetString(msg)
	SetUnexportedField(value.FieldByName("ignoreStats")).SetBool(ignoreStats)
	return ret
}

func GetWaterIfF(ifce *water.Interface) *os.File {
	// 获取 water tun 文件描述符
	// ifce.ReadWriteCloser.f as *os.File
	// 强制获取未公开成员
	var value reflect.Value
	if runtime.GOOS == "darwin" {
		value = reflect.ValueOf(ifce.ReadWriteCloser).Elem().FieldByName("f").Elem().Elem()
	} else if runtime.GOOS == "linux" {
		value = reflect.ValueOf(ifce.ReadWriteCloser).Elem()
	} else if runtime.GOOS == "windows" {
		// 这个不支持,只能获取到int fd:
		// reflect.ValueOf(ifce.ReadWriteCloser).Elem().FieldByName("fd").GetInt()
		value = reflect.ValueOf(ifce.ReadWriteCloser).Elem().FieldByName("fd").Elem().Elem()
	}
	valueI := (*os.File)(unsafe.Pointer(value.UnsafeAddr()))
	return valueI
}
