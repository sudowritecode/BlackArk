package main

import (
	"encoding/binary"
	"testing"
)

func frame(stream byte, payload string) []byte {
	b := make([]byte, 8+len(payload))
	b[0] = stream
	binary.BigEndian.PutUint32(b[4:8], uint32(len(payload)))
	copy(b[8:], payload)
	return b
}

func TestDecodeDockerLogs(t *testing.T) {
	framed := append(frame(1, "hello\n"), frame(2, "warning\n")...)
	if got, want := decodeDockerLogs(framed), "hello\nwarning\n"; got != want {
		t.Fatalf("decodeDockerLogs() = %q, want %q", got, want)
	}
}

func TestDecodeDockerLogsPlainText(t *testing.T) {
	if got, want := decodeDockerLogs([]byte("plain\n")), "plain\n"; got != want {
		t.Fatalf("decodeDockerLogs() = %q, want %q", got, want)
	}
}
