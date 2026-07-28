package main

import (
	"strings"
	"testing"
)

func TestFormatLANHints_WildcardBind(t *testing.T) {
	lines := formatLANHints(":8080", []string{"192.168.1.10", "10.0.0.5"})
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "http://127.0.0.1:8080") {
		t.Fatalf("missing dev URL:\n%s", joined)
	}
	if !strings.Contains(joined, "LAN: open http://<this-pc-ip>:8080") {
		t.Fatalf("missing LAN placeholder:\n%s", joined)
	}
	if !strings.Contains(joined, "try http://192.168.1.10:8080") {
		t.Fatalf("missing IPv4 try line:\n%s", joined)
	}
	if !strings.Contains(joined, "try http://10.0.0.5:8080") {
		t.Fatalf("missing second IPv4:\n%s", joined)
	}
}

func TestFormatLANHints_ZeroHost(t *testing.T) {
	lines := formatLANHints("0.0.0.0:9090", []string{"192.168.0.2"})
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "http://127.0.0.1:9090") {
		t.Fatalf("dev port: %s", joined)
	}
	if !strings.Contains(joined, "try http://192.168.0.2:9090") {
		t.Fatalf("try line: %s", joined)
	}
}

func TestFormatLANHints_NoIPv4(t *testing.T) {
	lines := formatLANHints(":8080", nil)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "no non-loopback IPv4") {
		t.Fatalf("want empty-IP note:\n%s", joined)
	}
}

func TestFormatLANHints_LoopbackOnly(t *testing.T) {
	lines := formatLANHints("127.0.0.1:8080", []string{"192.168.1.10"})
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "loopback only") {
		t.Fatalf("want loopback warning:\n%s", joined)
	}
	if strings.Contains(joined, "try http://192.168.1.10") {
		t.Fatalf("must not list LAN IPs when bound to loopback:\n%s", joined)
	}
}

func TestFormatLANHints_ExplicitLANHost(t *testing.T) {
	lines := formatLANHints("192.168.1.20:8080", nil)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "LAN: open http://192.168.1.20:8080") {
		t.Fatalf("want explicit LAN URL:\n%s", joined)
	}
}
