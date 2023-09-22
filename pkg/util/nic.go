package util

import (
	"net"

	log "github.com/sirupsen/logrus"
)

func GetNicFromIp(nic_ip net.IP) (str string, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}
	for _, i := range ifaces {
		//log.Infof("DEBUG checking: %s", i.Name)
		addrs, err := i.Addrs()
		if err == nil {
			for _, a := range addrs {
				ip, _, err := net.ParseCIDR(a.String())
				if err != nil {
					return "", err
				}
				// TODO: remove?
				log.Debugf("found interface %s with ip %s", i.Name, ip)
				if ip.String() == nic_ip.String() {
					return i.Name, err
				}
			}
		}
	}

	return
}

// TODO: remove
// func ListInterfaces() ([]string, error) {
// 	var ifs []string
// 	ifaces, err := net.Interfaces()
// 	if err != nil {
// 		return ifs, err
// 	}
// 	for _, i := range ifaces {
// 		ifs = append(ifs, i.Name)
// 	}

// 	return ifs, err
// }
