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

		port, err := parseLsofAddr(line)
		if err != nil {
			return nil, err
		}
		ports = append(ports, port)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan lsof output: %w", err)
	}

	return ports, nil
}

// parseLsofListeners parses `lsof -Fpcn` output, where a p (PID) line and a c
// (command) line precede the n (address) lines of that process's sockets.
func parseLsofListeners(output []byte) ([]Listener, error) {
	var listeners []Listener
	var pid int
	var command string

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		switch line[0] {
		case 'p':
			n, err := strconv.Atoi(line[1:])
			if err != nil {
				return nil, fmt.Errorf("parse lsof pid %q: %w", line, err)
			}
			pid, command = n, ""
		case 'c':
			command = line[1:]
		case 'n':
			port, err := parseLsofAddr(line)
			if err != nil {
				return nil, err
			}
			listeners = append(listeners, Listener{
				PID:      pid,
				Command:  command,
				Port:     port.Port,
				BindAddr: port.BindAddr,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan lsof output: %w", err)
	}

	return listeners, nil
}

// parseLsofAddr parses an lsof n line such as "n127.0.0.1:8080" or "n[::1]:8080".
func parseLsofAddr(line string) (Port, error) {
	host, portString, err := net.SplitHostPort(strings.TrimPrefix(line, "n"))
	if err != nil {
		return Port{}, fmt.Errorf("parse lsof listener %q: %w", line, err)
	}
	portNumber, err := strconv.Atoi(portString)
	if err != nil {
		return Port{}, fmt.Errorf("parse lsof listener port %q: %w", portString, err)
	}

	protocol := "tcp"
	bindAddr := host
	if strings.Contains(host, ":") {
		protocol = "tcp6"
	}
	if host == "*" {
		bindAddr = "0.0.0.0"
	}

	return Port{
		Port:     portNumber,
		Protocol: protocol,
		State:    "LISTEN",
		BindAddr: bindAddr,
	}, nil
}
