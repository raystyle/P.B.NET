package info

import (
	"net"
	"os"
	"os/user"
	"runtime"
)

type System struct {
	IPs      []string
	OS       string
	Arch     string
	PID      int
	Hostname string
	Username string
}

func GetSystemInfo() *System {
	system := System{}
	ifaces, err := net.Interfaces()
	if err == nil {
		for i := 0; i < len(ifaces); i++ {
			if ifaces[i].Flags == net.FlagUp|net.FlagBroadcast|net.FlagMulticast {
				addrs, err := ifaces[i].Addrs()
				if err != nil {
					continue
				}
				for j := 0; j < len(addrs); j++ {
					system.IPs = append(system.IPs, addrs[j].String())
				}
			}
		}
	}
	system.OS = runtime.GOOS
	system.Arch = runtime.GOARCH
	system.PID = os.Getpid()
	system.Hostname, err = os.Hostname()
	if err != nil {
		system.Hostname = "unknown"
	}
	u, err := user.Current()
	if err != nil {
		system.Username = "unknown"
	} else {
		system.Username = u.Username
	}
	return &system
}
