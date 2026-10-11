package agent

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/teawithsand/nworm/internal/config"
	"github.com/teawithsand/nworm/internal/keyutil"
	"github.com/teawithsand/nworm/internal/sshx"
)

func Run(cfg *config.Agent) error {
	if cfg.Name == "" {
		return fmt.Errorf("name is required")
	}
	if cfg.Identity == "" {
		return fmt.Errorf("identity is required")
	}
	if len(cfg.Relays) == 0 {
		return fmt.Errorf("at least one relay is required")
	}
	if cfg.Remote == "" {
		cfg.Remote = "127.0.0.1:22"
	}
	if cfg.Local == "" {
		cfg.Local = "127.0.0.1:22"
	}
	keepalive := cfg.Keepalive.D()
	if keepalive == 0 {
		keepalive = 30 * time.Second
	}
	signer, err := keyutil.LoadSigner(cfg.Identity)
	if err != nil {
		return fmt.Errorf("identity: %w", err)
	}
	cb, err := keyutil.HostKeyCallback(cfg.KnownHosts, cfg.Insecure)
	if err != nil {
		return fmt.Errorf("host key: %w", err)
	}
	clientCfg := &ssh.ClientConfig{
		User:            "agent-" + cfg.Name,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: cb,
		Timeout:         15 * time.Second,
	}
	var wg sync.WaitGroup
	for _, relay := range cfg.Relays {
		wg.Add(1)
		go func(relay string) {
			defer wg.Done()
			supervise(relay, cfg, clientCfg, keepalive)
		}(relay)
	}
	wg.Wait()
	return nil
}

func supervise(relay string, cfg *config.Agent, clientCfg *ssh.ClientConfig, keepalive time.Duration) {
	backoff := time.Second
	for {
		start := time.Now()
		err := session(relay, cfg, clientCfg, keepalive)
		if err != nil {
			log.Printf("agent %q relay %s: %v", cfg.Name, relay, err)
		}
		if time.Since(start) > time.Minute {
			backoff = time.Second
		}
		time.Sleep(backoff)
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func session(relay string, cfg *config.Agent, clientCfg *ssh.ClientConfig, keepalive time.Duration) error {
	client, err := sshx.Dial(relay, clientCfg)
	if err != nil {
		return err
	}
	defer client.Close()
	go sshx.Keepalive(client, keepalive)

	ln, err := client.Listen("tcp", cfg.Remote)
	if err != nil {
		return fmt.Errorf("register %s: %w", cfg.Remote, err)
	}
	log.Printf("agent %q registered %s via %s", cfg.Name, cfg.Remote, relay)
	for {
		c, err := ln.Accept()
		if err != nil {
			return err
		}
		go handle(c, cfg.Local)
	}
}

func handle(c net.Conn, local string) {
	defer c.Close()
	t, err := net.DialTimeout("tcp", local, 10*time.Second)
	if err != nil {
		log.Printf("dial local %s: %v", local, err)
		return
	}
	defer t.Close()
	sshx.Bridge(c, t)
}
