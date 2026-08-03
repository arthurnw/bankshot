package monitor

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"
)

func parseLsofListeningPorts(output []byte) ([]Port, error) {
	var ports []Port
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "n") {
			continue
		}

		host, portString, err := net.SplitHostPort(strings.TrimPrefix(line, "n"))
		if err != nil {
			return nil, fmt.Errorf("parse lsof listener %q: %w", line, err)
		}
		portNumber, err := strconv.Atoi(portString)
		if err != nil {
			return nil, fmt.Errorf("parse lsof listener port %q: %w", portString, err)
		}

		protocol := "tcp"
		bindAddr := host
		if strings.Contains(host, ":") {
			protocol = "tcp6"
		}
		if host == "*" {
			bindAddr = "0.0.0.0"
		}

		ports = append(ports, Port{
			Port:     portNumber,
			Protocol: protocol,
			State:    "LISTEN",
			BindAddr: bindAddr,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan lsof output: %w", err)
	}

	return ports, nil
}
