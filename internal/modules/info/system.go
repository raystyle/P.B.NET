package info

import (
	"net"
	"os"
	"os/user"
	"runtime"
	"strings"
)

type System struct {
	IP        []string // 192.168.1.11/24, fe80::5456:5f8:1690:5792/64
	OS        string   // windows, linux
	Arch      string   // amd64, 386
	GoVersion string   // go1.13.5 -> 1.13.5
	PID       int      // 2000
	PPID      int      // 1999
	Hostname  string   // WIN-F0F2A61229S
	Username  string   // WIN-F0F2A61229S\Admin
}

func GetSystemInfo() *System {
	system := System{}
	ifaces, _ := net.Interfaces()
	for i := 0; i < len(ifaces); i++ {
		if ifaces[i].Flags == net.FlagUp|net.FlagBroadcast|net.FlagMulticast {
			addrs, _ := ifaces[i].Addrs()
			for j := 0; j < len(addrs); j++ {
				system.IP = append(system.IP, addrs[j].String())
			}
		}
	}
	system.OS = runtime.GOOS
	system.Arch = runtime.GOARCH
	system.PID = os.Getpid()
	system.PPID = os.Getppid()
	system.GoVersion = strings.Split(runtime.Version(), "go")[1]
	system.Hostname, _ = os.Hostname()
	u, err := user.Current()
	if err == nil {
		system.Username = u.Username
	}
	return &system
}
