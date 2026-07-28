package main

import (
	"fmt"
	"net"
	"os"
	"strings"
)

// formatLANHints returns startup lines telling the learner how to open the app
// from a phone on the same Wi‑Fi (and local dev URL). Pure for tests.
func formatLANHints(addr string, ipv4s []string) []string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		if strings.HasPrefix(addr, ":") {
			port = strings.TrimPrefix(addr, ":")
			host = ""
		} else {
			return []string{fmt.Sprintf("Open the app at http://%s (set MINER_ADDR host:port if needed)", addr)}
		}
	}
	if port == "" {
		port = "8080"
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Dev: http://127.0.0.1:%s", port))

	switch {
	case host == "" || host == "0.0.0.0" || host == "::" || host == "[::]":
		lines = append(lines, fmt.Sprintf("LAN: open http://<this-pc-ip>:%s from your phone on the same Wi‑Fi", port))
		for _, ip := range ipv4s {
			if ip == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("  try http://%s:%s", ip, port))
		}
		if len(ipv4s) == 0 {
			lines = append(lines, "  (no non-loopback IPv4 found; check Wi‑Fi / firewall)")
		}
	case host == "127.0.0.1" || host == "localhost" || host == "::1":
		lines = append(lines, "Listening on loopback only — phone on LAN cannot reach this process.")
		lines = append(lines, "Bind all interfaces for phone use: MINER_ADDR=:8080")
	default:
		h := strings.Trim(host, "[]")
		lines = append(lines, fmt.Sprintf("LAN: open http://%s:%s from your phone on the same Wi‑Fi", h, port))
	}
	return lines
}

// listNonLoopbackIPv4 collects up IPv4 addresses suitable for phone LAN URLs.
func listNonLoopbackIPv4() []string {
	var out []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if v4 := ip.To4(); v4 != nil {
				out = append(out, v4.String())
			}
		}
	}
	return out
}

func logLANHints(addr string) {
	for _, line := range formatLANHints(addr, listNonLoopbackIPv4()) {
		fmt.Fprintln(os.Stderr, line)
	}
}
