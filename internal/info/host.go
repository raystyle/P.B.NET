package info

import (
	"net"
	"os"
	"os/user"
	"runtime"
)

type HostInfo struct {
	IPs      []string
	OS       string
	Hostname string
	Username string
	PID      int
}

func Host() HostInfo {
	hostInfo := HostInfo{}
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
	hostInfo.OS = runtime.GOOS
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
	return hostInfo
}
