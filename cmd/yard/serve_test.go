package main

import "testing"

func TestValidateYardServeBindAllowsLoopbackHosts(t *testing.T) {
	for _, host := range []string{"localhost", "LOCALHOST", "127.0.0.1", "::1", "[::1]"} {
		t.Run(host, func(t *testing.T) {
			if err := validateYardServeBind(host, false); err != nil {
				t.Fatalf("validateYardServeBind(%q, false) returned error: %v", host, err)
			}
		})
	}
}

func TestValidateYardServeBindRejectsNonLoopbackHostsWithoutOptIn(t *testing.T) {
	for _, host := range []string{"", "0.0.0.0", "::", "192.168.1.20", "example.com"} {
		t.Run(host, func(t *testing.T) {
			if err := validateYardServeBind(host, false); err == nil {
				t.Fatalf("validateYardServeBind(%q, false) returned nil, want error", host)
			}
		})
	}
}

func TestValidateYardServeBindAllowsNonLoopbackHostsWithOptIn(t *testing.T) {
	for _, host := range []string{"", "0.0.0.0", "::", "192.168.1.20", "example.com"} {
		t.Run(host, func(t *testing.T) {
			if err := validateYardServeBind(host, true); err != nil {
				t.Fatalf("validateYardServeBind(%q, true) returned error: %v", host, err)
			}
		})
	}
}
