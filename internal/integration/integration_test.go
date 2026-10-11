package integration

import (
	"io"
	"strconv"
	"testing"
	"time"
)

func TestEndToEndThroughRelay(t *testing.T) {
	b := startBench(t, nil)
	b.waitReachable(t, "admin", b.adminKey)

	client, err := b.dial(t, "admin", b.adminKey)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	conn, err := client.Dial("tcp", "test:"+strconv.Itoa(b.echoPort))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("ping\n")); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 5)
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "ping\n" {
		t.Fatalf("echo = %q", buf)
	}
}
