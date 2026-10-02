// Package logging builds the slog loggers used by bankshotd and the monitor,
// optionally writing to a size-capped log file.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/mitchellh/go-homedir"
)

// DefaultMaxBytes is the size at which a log file is rotated.
const DefaultMaxBytes = 10 << 20

// ParseLevel maps a config log level to a slog level, defaulting to info.
func ParseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// New returns a text logger at level. It writes to path, rotated at
// DefaultMaxBytes, or to stderr when path is empty.
func New(level slog.Level, path string) (*slog.Logger, error) {
	var w io.Writer = os.Stderr
	if path != "" {
		f, err := OpenRotating(path, DefaultMaxBytes)
		if err != nil {
			return nil, err
		}
		w = f
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})), nil
}

// RotatingFile appends to a log file. When a write would take the file past
// maxBytes, it renames the file to <path>.1, replacing any older copy, and
// starts a new one. A process that writes its own log can rotate it this way;
// launchd keeps its StandardErrorPath descriptor open, so renaming that file
// from outside does not stop it growing.
type RotatingFile struct {
	path     string
	maxBytes int64

	mu   sync.Mutex
	file *os.File
	size int64
}

// OpenRotating opens path for appending, creating it and its directory if
// needed. A leading ~ is expanded.
func OpenRotating(path string, maxBytes int64) (*RotatingFile, error) {
	expanded, err := homedir.Expand(path)
	if err != nil {
		return nil, fmt.Errorf("expand log path %q: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(expanded), 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	r := &RotatingFile{path: expanded, maxBytes: maxBytes}
	if err := r.open(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *RotatingFile) open() error {
	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return fmt.Errorf("stat log file: %w", err)
	}
	r.file, r.size = f, info.Size()
	return nil
}

func (r *RotatingFile) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.size > 0 && r.size+int64(len(p)) > r.maxBytes {
		if err := r.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := r.file.Write(p)
	r.size += int64(n)
	return n, err
}

func (r *RotatingFile) rotate() error {
	if err := r.file.Close(); err != nil {
		return fmt.Errorf("close log file: %w", err)
	}
	if err := os.Rename(r.path, r.path+".1"); err != nil {
		return fmt.Errorf("rotate log file: %w", err)
	}
	return r.open()
}

// Close closes the current log file.
func (r *RotatingFile) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.file.Close()
}
