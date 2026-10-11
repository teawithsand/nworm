package sshx

import (
	"io"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

func Dial(addr string, cfg *ssh.ClientConfig) (*ssh.Client, error) {
	nc, err := net.DialTimeout("tcp", addr, 15*time.Second)
	if err != nil {
		return nil, err
	}
	conn, chans, reqs, err := ssh.NewClientConn(nc, addr, cfg)
	if err != nil {
		nc.Close()
		return nil, err
	}
	return ssh.NewClient(conn, chans, reqs), nil
}

func Bridge(a, b io.ReadWriteCloser) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = io.Copy(a, b); halfCloseWrite(a) }()
	go func() { defer wg.Done(); _, _ = io.Copy(b, a); halfCloseWrite(b) }()
	wg.Wait()
	a.Close()
	b.Close()
}

func halfCloseWrite(c io.ReadWriteCloser) {
	if cw, ok := c.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
		return
	}
	_ = c.Close()
}

func Keepalive(conn ssh.Conn, every time.Duration) {
	if every <= 0 {
		return
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for range t.C {
		if _, _, err := conn.SendRequest("keepalive@nworm", true, nil); err != nil {
			conn.Close()
			return
		}
	}
}
