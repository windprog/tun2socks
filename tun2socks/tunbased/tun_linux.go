package tunbased

import (
	"fmt"
	"log"
	"net"

	"github.com/songgao/water"
)

func Ifconfig(tunName, network string, _ uint32) {
	var ip, ipv4Net, _ = net.ParseCIDR(network)
	ipStr := ip.To4().String()
	sargs := fmt.Sprintf("%s %s netmask %s", tunName, ipStr, Ipv4MaskString(ipv4Net.Mask))
	if err := ExecCommand("ifconfig", sargs); err != nil {
		log.Fatal("execCommand failed", err)
	}
}

func GetCfg() water.Config {
	return water.Config{
		DeviceType: water.TUN,
	}
}
