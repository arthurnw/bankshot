package cli

import (
	"reflect"
	"testing"

	"github.com/phinze/bankshot/pkg/monitor"
)

func TestLoopbackPorts(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want []int
	}{
		{"localhost url", "http://localhost:3000/app", []int{3000}},
		{"ipv4 loopback", "http://127.0.0.1:8080", []int{8080}},
		{"ipv6 loopback", "http://[::1]:8080", []int{8080}},
		{
			"oauth redirect_uri",
			"https://accounts.example.com/authorize?client_id=x&redirect_uri=http%3A%2F%2Flocalhost%3A53682%2Fcallback&state=y",
			[]int{53682},
		},
		{
			"loopback url with matching redirect_uri",
			"http://localhost:9000/login?redirect_uri=http://localhost:9000/cb",
			[]int{9000},
		},
		{"remote host", "https://github.com:8443/login", nil},
		{"remote redirect_uri", "https://idp.example.com/auth?redirect_uri=https://app.example.com/cb", nil},
		{"no explicit port", "http://localhost/callback", nil},
		{"privileged port", "http://localhost:80", nil},
		{"non-http scheme", "ftp://localhost:2121", nil},
		{"unparseable", "http://[::1", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := loopbackPorts(tt.url)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("loopbackPorts(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestListeningPorts(t *testing.T) {
	listening := []monitor.Port{
		{Port: 3000, BindAddr: "127.0.0.1"},
		{Port: 4000, BindAddr: "0.0.0.0"},
		{Port: 5000, BindAddr: "192.168.1.10"},
	}

	got := listeningPorts([]int{3000, 4000, 5000, 6000}, listening)
	want := []int{3000, 4000}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("listeningPorts() = %v, want %v", got, want)
	}
}

func TestNormalizeOpenURL(t *testing.T) {
	tests := []struct{ in, want string }{
		{"3000", "http://localhost:3000"},
		{"localhost:18702", "http://localhost:18702"},
		{"localhost:3000/app?x=1", "http://localhost:3000/app?x=1"},
		{"127.0.0.1:8080", "http://127.0.0.1:8080"},
		{"[::1]:8080/", "http://[::1]:8080/"},
		{"0.0.0.0:5173", "http://localhost:5173"},
		{"http://0.0.0.0:5173/app?x=1", "http://localhost:5173/app?x=1"},
		{"http://[::]:8000/", "http://localhost:8000/"},
		{"https://0.0.0.0", "https://localhost"},
		{"localhost", "http://localhost"},
		{"http://localhost:3000", "http://localhost:3000"},
		{"https://github.com", "https://github.com"},
		{"about:blank", "about:blank"},
		{"mailto:a@example.com", "mailto:a@example.com"},
		{"localhostfoo:3000", "localhostfoo:3000"},
		{"70000", "70000"},
	}
	for _, tt := range tests {
		if got := normalizeOpenURL(tt.in); got != tt.want {
			t.Errorf("normalizeOpenURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
