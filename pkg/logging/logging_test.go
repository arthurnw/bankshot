package logging

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRotatingFileRotatesAtLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "bankshotd.log")
	r, err := OpenRotating(path, 25)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	// Lines are 11-12 bytes, so each file holds two before the next rotation.
	for _, line := range []string{"first line\n", "second line\n", "third line\n", "fourth line\n", "fifth line\n", "sixth line\n"} {
		if _, err := r.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}

	current, _ := os.ReadFile(path)
	previous, _ := os.ReadFile(path + ".1")
	if string(current) != "fifth line\nsixth line\n" {
		t.Errorf("current log = %q, want the last two lines", current)
	}
	// The second rotation replaced the first rotated copy.
	if string(previous) != "third line\nfourth line\n" {
		t.Errorf("rotated log = %q, want the two lines before those", previous)
	}
}

func TestRotatingFileAppendsToExistingLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bankshotd.log")
	if err := os.WriteFile(path, []byte("before restart\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := OpenRotating(path, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Write([]byte("after restart\n")); err != nil {
		t.Fatal(err)
	}
	r.Close()

	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, []byte("before restart\nafter restart\n")) {
		t.Errorf("log = %q", got)
	}
}

func TestOversizedWriteStillLands(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bankshotd.log")
	r, err := OpenRotating(path, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	if _, err := r.Write([]byte("longer than the limit\n")); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "longer than the limit\n" {
		t.Errorf("log = %q", got)
	}
}
