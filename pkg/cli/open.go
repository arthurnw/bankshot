package cli

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"

	"github.com/google/uuid"
	"github.com/phinze/bankshot/pkg/monitor"
	"github.com/phinze/bankshot/pkg/protocol"
	"github.com/spf13/cobra"
)

func newOpenCmd() *cobra.Command {
	var noForward bool

	cmd := &cobra.Command{
		Use:   "open [url]",
		Short: "Open a URL in the local browser",
		Long: `Opens the specified URL in the default browser on the local machine.

Before opening, any loopback port the URL points at, directly or through an
OAuth redirect_uri parameter, is forwarded if something on this machine is
listening on it. A CLI login flow starts its callback listener before it opens
the browser, so the forward exists before the provider redirects back.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rawURL := args[0]

			if !noForward {
				forwardOpenedPorts(rawURL)
			}

			openReq := protocol.OpenRequest{URL: rawURL}
			payload, err := json.Marshal(openReq)
			if err != nil {
				return fmt.Errorf("failed to marshal request: %w", err)
			}

			req := protocol.Request{
				ID:      uuid.New().String(),
				Type:    protocol.CommandOpen,
				Payload: payload,
			}

			resp, err := sendRequest(&req)
			if err != nil {
				return err
			}

			if !resp.Success {
				return fmt.Errorf("failed to open URL: %s", resp.Error)
			}

			if verbose {
				fmt.Println("URL opened successfully")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&noForward, "no-forward", false, "Do not forward loopback ports referenced by the URL")

	return cmd
}

// forwardOpenedPorts forwards the listening loopback ports referenced by rawURL.
// Failures are reported but do not prevent opening the URL.
func forwardOpenedPorts(rawURL string) {
	candidates := loopbackPorts(rawURL)
	if len(candidates) == 0 {
		return
	}

	listening, err := monitor.GetListeningPorts()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bankshot: could not check listening ports: %v\n", err)
		return
	}

	ports := listeningPorts(candidates, listening)
	if len(ports) == 0 {
		return
	}

	hostname, err := os.Hostname()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bankshot: could not get hostname: %v\n", err)
		return
	}

	for _, port := range ports {
		if err := requestForward(hostname, port); err != nil {
			fmt.Fprintf(os.Stderr, "bankshot: could not forward port %d: %v\n", port, err)
		} else if verbose {
			fmt.Printf("Forwarded port %d before opening URL\n", port)
		}
	}
}

func requestForward(connectionInfo string, port int) error {
	payload, err := json.Marshal(protocol.ForwardRequest{
		RemotePort:     port,
		LocalPort:      port,
		Host:           "localhost",
		ConnectionInfo: connectionInfo,
	})
	if err != nil {
		return err
	}

	resp, err := sendRequest(&protocol.Request{
		ID:      uuid.New().String(),
		Type:    protocol.CommandForward,
		Payload: payload,
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

// loopbackPorts returns the explicit loopback ports in rawURL and in its
// redirect_uri query parameter, without duplicates.
func loopbackPorts(rawURL string) []int {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}

	var ports []int
	seen := make(map[int]bool)
	add := func(u *url.URL) {
		port, ok := loopbackPort(u)
		if ok && !seen[port] {
			seen[port] = true
			ports = append(ports, port)
		}
	}

	add(u)
	if redirect := u.Query().Get("redirect_uri"); redirect != "" {
		if ru, err := url.Parse(redirect); err == nil {
			add(ru)
		}
	}
	return ports
}

func loopbackPort(u *url.URL) (int, bool) {
	if u.Scheme != "http" && u.Scheme != "https" {
		return 0, false
	}

	host := u.Hostname()
	if host != "localhost" {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return 0, false
		}
	}

	// Without an explicit port the URL targets 80 or 443, which are not
	// forwarded.
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1024 || port > 65535 {
		return 0, false
	}
	return port, true
}

// listeningPorts returns the candidates that have a locally reachable listener.
func listeningPorts(candidates []int, listening []monitor.Port) []int {
	open := make(map[int]bool)
	for _, p := range listening {
		if monitor.IsLocalAddr(p.BindAddr) {
			open[p.Port] = true
		}
	}

	var ports []int
	for _, port := range candidates {
		if open[port] {
			ports = append(ports, port)
		}
	}
	return ports
}
