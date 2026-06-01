//go:build !darwin

package notify

import (
	"log/slog"
	"os/exec"
)

// New creates a Notifier. If helperPath is empty, notifications are disabled.
func New(logger *slog.Logger, helperPath string) *Notifier {
	return &Notifier{
		logger:     logger,
		helperPath: helperPath,
	}
}

// send invokes the notification helper in a goroutine so it never blocks the
// caller.
func (n *Notifier) send(args ...string) {
	go func() {
		cmd := exec.Command(n.helperPath, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			n.logger.Warn("notification helper failed",
				"error", err,
				"output", string(out),
				"helper", n.helperPath,
			)
		} else {
			n.logger.Debug("Notification helper succeeded",
				"output", string(out),
			)
		}
	}()
}
