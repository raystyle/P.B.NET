package info

import (
	"net"
	"os"
	"os/user"
	"runtime"
	"strings"
)

// System contains the current system information.
type System struct {
	IP        []string `json:"ip"`         // 192.168.1.11/24, fe80::5456:5f8:1690:5792/64
	OS        string   `json:"os"`         // windows, linux
	Arch      string   `json:"arch"`       // amd64, 386
	GoVersion string   `json:"go_version"` // go1.13.5 -> 1.13.5
	PID       int      `json:"pid"`        // 2000
	PPID      int      `json:"ppid"`       // 1999
	Hostname  string   `json:"hostname"`   // WIN-F0F2A61229S
	Username  string   `json:"username"`   // WIN-F0F2A61229S\Admin
}

// GetSystemInfo is used to get current system information.
func GetSystemInfo() *System {
	system := System{}
	ifaces, _ := net.Interfaces()
	for i := 0; i < len(ifaces); i++ {
		if ifaces[i].Flags == net.FlagUp|net.FlagBroadcast|net.FlagMulticast {
			addresses, _ := ifaces[i].Addrs()
			for j := 0; j < len(addresses); j++ {
				system.IP = append(system.IP, addresses[j].String())
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
