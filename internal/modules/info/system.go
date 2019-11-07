package info

import (
	"net"
	"os"
	"os/user"
	"runtime"
)

type System struct {
	IP        []string
	OS        string
	Arch      string
	GoVersion string
	PID       int
	PPID      int
	Hostname  string
	Username  string
}

func GetSystemInfo() *System {
	system := System{}
	ifaces, _ := net.Interfaces()
	for i := 0; i < len(ifaces); i++ {
		if ifaces[i].Flags == net.FlagUp|net.FlagBroadcast|net.FlagMulticast {
			addrs, err := ifaces[i].Addrs()
			if err != nil {
				continue
			}
			for j := 0; j < len(addrs); j++ {
				system.IP = append(system.IP, addrs[j].String())
			}
		}
	}
	system.OS = runtime.GOOS
	system.Arch = runtime.GOARCH
	system.PID = os.Getpid()
	system.PPID = os.Getppid()
	system.GoVersion = runtime.Version()
	var err error
	system.Hostname, err = os.Hostname()
	if err != nil {
		system.Hostname = "<unknown>"
	}
	u, err := user.Current()
	if err != nil {
		system.Username = "<unknown>"
	} else {
		system.Username = u.Username
	}
	return &system
}
