package share

import (
	"log"
	"net"
)

var localIpV4 string

func init() {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Fatalln("check net interfaces error:" + err.Error())
	}
	for _, address := range addrs {
		// 检查ip地址判断是否回环地址
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				localIpV4 = ipnet.IP.String()
				break
			}
		}
	}
}

func LocalIpV4() string {
	return localIpV4
}
