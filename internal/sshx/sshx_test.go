package sshx

import (
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestDialConnectionRefused(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	if _, err := Dial(addr, &ssh.ClientConfig{
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         time.Second,
	}); err == nil {
		t.Fatal("expected Dial to fail against a closed port")
	}
}

func TestKeepaliveDisabled(t *testing.T) {
	// A non-positive interval must return immediately without touching the
	// connection; passing nil proves the connection is never dereferenced.
	Keepalive(nil, 0)
	Keepalive(nil, -time.Second)
}

func TestKeepaliveClosesOnError(t *testing.T) {
	conn := &stubConn{}
	go Keepalive(conn, time.Millisecond)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn.mu.Lock()
		closed := conn.closed
		conn.mu.Unlock()
		if closed {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if !conn.closed {
		t.Fatal("Keepalive did not close the connection after a SendRequest error")
	}
	if conn.sent == 0 {
		t.Fatal("Keepalive never sent a keepalive request")
	}
}

type stubConn struct {
	mu     sync.Mutex
	sent   int
	closed bool
}

func (s *stubConn) User() string          { return "stub" }
func (s *stubConn) SessionID() []byte     { return nil }
func (s *stubConn) ClientVersion() []byte { return []byte("SSH-2.0-stub") }
func (s *stubConn) ServerVersion() []byte { return []byte("SSH-2.0-stub") }
func (s *stubConn) RemoteAddr() net.Addr  { return stubAddr{} }
func (s *stubConn) LocalAddr() net.Addr   { return stubAddr{} }
func (s *stubConn) SendRequest(string, bool, []byte) (bool, []byte, error) {
	s.mu.Lock()
	s.sent++
	s.mu.Unlock()
	return false, nil, errors.New("keepalive failed")
}
func (s *stubConn) OpenChannel(string, []byte) (ssh.Channel, <-chan *ssh.Request, error) {
	return nil, nil, errors.New("no channels")
}
func (s *stubConn) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	return nil
}
func (s *stubConn) Wait() error { return nil }

type stubAddr struct{}

func (stubAddr) Network() string { return "tcp" }
func (stubAddr) String() string  { return "127.0.0.1:1" }

func TestBridgeBidirectional(t *testing.T) {
	a1, a2 := net.Pipe()
	b1, b2 := net.Pipe()
	done := make(chan struct{})
	go func() {
		Bridge(a1, b1)
		close(done)
	}()

	go func() { _, _ = b2.Write([]byte("from-b")) }()
	fromB := make([]byte, len("from-b"))
	_ = a2.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.ReadFull(a2, fromB); err != nil {
		t.Fatalf("a-side did not receive b-side data: %v", err)
	}
	if string(fromB) != "from-b" {
		t.Fatalf("a-side received %q", fromB)
	}

	go func() { _, _ = a2.Write([]byte("from-a")) }()
	fromA := make([]byte, len("from-a"))
	_ = b2.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.ReadFull(b2, fromA); err != nil {
		t.Fatalf("b-side did not receive a-side data: %v", err)
	}
	if string(fromA) != "from-a" {
		t.Fatalf("b-side received %q", fromA)
	}

	a2.Close()
	b2.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Bridge did not return after both peers closed")
	}
}

type closeWriter struct {
	closeCalls      int
	closeWriteCalls int
}

func (c *closeWriter) Read([]byte) (int, error)    { return 0, io.EOF }
func (c *closeWriter) Write(p []byte) (int, error) { return len(p), nil }
func (c *closeWriter) Close() error                { c.closeCalls++; return nil }
func (c *closeWriter) CloseWrite() error           { c.closeWriteCalls++; return nil }

type plainCloser struct {
	closeCalls int
}

func (c *plainCloser) Read([]byte) (int, error)    { return 0, io.EOF }
func (c *plainCloser) Write(p []byte) (int, error) { return len(p), nil }
func (c *plainCloser) Close() error                { c.closeCalls++; return nil }

func TestHalfCloseWritePrefersCloseWrite(t *testing.T) {
	c := &closeWriter{}
	halfCloseWrite(c)
	if c.closeWriteCalls != 1 {
		t.Fatalf("CloseWrite calls = %d, want 1", c.closeWriteCalls)
	}
	if c.closeCalls != 0 {
		t.Fatalf("Close calls = %d, want 0 when CloseWrite is available", c.closeCalls)
	}
}

func TestHalfCloseWriteFallsBackToClose(t *testing.T) {
	c := &plainCloser{}
	halfCloseWrite(c)
	if c.closeCalls != 1 {
		t.Fatalf("Close calls = %d, want 1", c.closeCalls)
	}
}
