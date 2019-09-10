package info

import (
	"net"
	"os"
	"os/user"
	"sync"
)

type HostInfo struct {
	IPs      []string
	Hostname string
	Username string
	PID      int
}

var (
	hostInfo        HostInfo
	getHostInfoOnce sync.Once
)

func Host() HostInfo {
	getHostInfoOnce.Do(func() {
		ifaces, err := net.Interfaces()
		if err == nil {
			var ips []string
			for i := 0; i < len(ifaces); i++ {
				if ifaces[i].Flags == net.FlagUp|net.FlagBroadcast|net.FlagMulticast {
					addrs, err := ifaces[i].Addrs()
					if err != nil {
						continue
					}
					for j := 0; j < len(addrs); j++ {
						ips = append(ips, addrs[j].String())
					}
				}
			}
			hostInfo.IPs = ips
		}
		hn, err := os.Hostname()
		if err != nil {
			hostInfo.Hostname = "unknown"
		} else {
			hostInfo.Hostname = hn
		}
		u, err := user.Current()
		if err != nil {
			hostInfo.Username = "unknown"
		} else {
			hostInfo.Username = u.Username
		}
		hostInfo.PID = os.Getpid()
	})
	return hostInfo
}
