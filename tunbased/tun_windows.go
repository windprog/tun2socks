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
	sargs := fmt.Sprintf("interface ip set address \"%s\" static %s %s none", tunName, ipStr, Ipv4MaskString(ipv4Net.Mask))
	if err := ExecCommand("netsh", sargs); err != nil {
		log.Fatal("execCommand failed", err)
	}
}

func GetCfg() water.Config {
	return water.Config{
		DeviceType: water.TUN,
		PlatformSpecificParams: water.PlatformSpecificParams{
			ComponentID: "tap0901",
			Network:     app.Cfg.General.Network,
		},
	}
}
