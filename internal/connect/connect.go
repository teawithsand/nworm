package connect

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/teawithsand/nworm/internal/config"
	"github.com/teawithsand/nworm/internal/keyutil"
	"github.com/teawithsand/nworm/internal/sshx"
)

// resolvePort applies the port precedence:
// explicit --port > dest[name] > port > 22.
func resolvePort(cfg *config.Connect, name string, flagPort uint32) uint32 {
	if flagPort != 0 {
		return flagPort
	}
	if p := cfg.Dest[name]; p != 0 {
		return p
	}
	if cfg.Port != 0 {
		return cfg.Port
	}
	return 22
}

func Run(cfg *config.Connect, name string, port uint32, listen string) error {
	if name == "" {
		return fmt.Errorf("target name is required")
	}
	if cfg.AdminUser == "" {
		return fmt.Errorf("admin_user is required")
	}
	if cfg.Identity == "" {
		return fmt.Errorf("identity is required")
	}
	if len(cfg.Relays) == 0 {
		return fmt.Errorf("at least one relay is required")
	}
	port = resolvePort(cfg, name, port)
	signer, err := keyutil.LoadSigner(cfg.Identity)
	if err != nil {
		return fmt.Errorf("identity: %w", err)
	}
	cb, err := keyutil.HostKeyCallback(cfg.KnownHosts, cfg.Insecure)
	if err != nil {
		return fmt.Errorf("host key: %w", err)
	}
	clientCfg := &ssh.ClientConfig{
		User:            cfg.AdminUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: cb,
		Timeout:         15 * time.Second,
	}
	dest := fmt.Sprintf("%s:%d", name, port)
	var lastErr error
	for _, relay := range cfg.Relays {
		client, err := sshx.Dial(relay, clientCfg)
		if err != nil {
			log.Printf("relay %s: %v", relay, err)
			lastErr = err
			continue
		}
		if listen != "" {
			return serve(client, dest, listen)
		}
		log.Printf("connected %s via %s", dest, relay)
		conn, err := client.Dial("tcp", dest)
		if err != nil {
			client.Close()
			lastErr = err
			log.Printf("relay %s: %v", relay, err)
			continue
		}
		go func() {
			_, _ = io.Copy(conn, os.Stdin)
			if cw, ok := conn.(interface{ CloseWrite() error }); ok {
				_ = cw.CloseWrite()
			} else {
				conn.Close()
			}
		}()
		_, _ = io.Copy(os.Stdout, conn)
		conn.Close()
		client.Close()
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no reachable relay")
	}
	return fmt.Errorf("all relays failed: %w", lastErr)
}

func serve(client *ssh.Client, dest, listen string) error {
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	log.Printf("listening on %s -> %s", listen, dest)
	for {
		local, err := ln.Accept()
		if err != nil {
			return err
		}
		go func(local net.Conn) {
			defer local.Close()
			remote, err := client.Dial("tcp", dest)
			if err != nil {
				log.Printf("dial %s: %v", dest, err)
				return
			}
			defer remote.Close()
			sshx.Bridge(local, remote)
		}(local)
	}
}
