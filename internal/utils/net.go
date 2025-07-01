package utils

import (
	"errors"
	"net"
	"strconv"
	"strings"
)

// GetLocalIP returns the first non-loopback IP address
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "localhost"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "localhost"
}

// NormalizeAddress formats address for gRPC connections
func NormalizeAddress(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "0.0.0.0" + addr
	}
	return addr
}

// IsPortAvailable checks if a TCP port is available
func IsPortAvailable(port int) bool {
	addr := net.JoinHostPort("localhost", strconv.Itoa(port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// FindAvailablePort finds an available TCP port in range
func FindAvailablePort(start, end int) (int, error) {
	for port := start; port <= end; port++ {
		if IsPortAvailable(port) {
			return port, nil
		}
	}
	return 0, errors.New("no available ports in range")
}
