package relay

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/teawithsand/nworm/internal/config"
	"github.com/teawithsand/nworm/internal/keyutil"
	"github.com/teawithsand/nworm/internal/sshx"
)

type acl struct {
	keys  []ssh.PublicKey
	allow map[string]bool
}

// allowed reports whether this admin may route to host. A nil or empty allow
// list denies everything; use the explicit "*" entry to allow every agent.
func (a *acl) allowed(host string) bool {
	if a == nil {
		return false
	}
	return a.allow["*"] || a.allow[host]
}

type agentConn struct {
	name  string
	conn  *ssh.ServerConn
	mu    sync.Mutex
	ports map[uint32]string
}

type server struct {
	sshCfg    *ssh.ServerConfig
	agentKeys map[string][]ssh.PublicKey
	admins    map[string]*acl
	agents    map[string]*agentConn
	keepalive time.Duration
	mu        sync.RWMutex
}

type forwardedTCPPayload struct {
	Addr       string
	Port       uint32
	OriginAddr string
	OriginPort uint32
}

type tcpipForwardRequest struct {
	Addr string
	Port uint32
}

type directTCPIPData struct {
	Host       string
	Port       uint32
	OriginHost string
	OriginPort uint32
}

func Run(cfg *config.Relay) error {
	if cfg.Listen == "" {
		cfg.Listen = ":443"
	}
	hostKey, err := keyutil.LoadOrCreateHostKey(cfg.HostKey)
	if err != nil {
		return fmt.Errorf("host key: %w", err)
	}
	s := &server{
		agentKeys: map[string][]ssh.PublicKey{},
		admins:    map[string]*acl{},
		agents:    map[string]*agentConn{},
		keepalive: cfg.Keepalive.D(),
	}
	if s.keepalive == 0 {
		s.keepalive = 30 * time.Second
	}
	for name, a := range cfg.Agents {
		keys, err := keyutil.ParseAuthorizedKeys(a.Keys)
		if err != nil {
			return fmt.Errorf("agent %q: %w", name, err)
		}
		s.agentKeys[name] = keys
	}
	for user, a := range cfg.Admins {
		keys, err := keyutil.ParseAuthorizedKeys(a.Keys)
		if err != nil {
			return fmt.Errorf("admin %q: %w", user, err)
		}
		allow := map[string]bool{}
		for _, n := range a.Allow {
			allow[n] = true
		}
		s.admins[user] = &acl{keys: keys, allow: allow}
	}
	s.sshCfg = &ssh.ServerConfig{
		ServerVersion:     "SSH-2.0-nworm",
		MaxAuthTries:      3,
		PublicKeyCallback: s.auth,
	}
	s.sshCfg.AddHostKey(hostKey)

	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return err
	}
	log.Printf("relay listening on %s (host key %s)", cfg.Listen, ssh.FingerprintSHA256(hostKey.PublicKey()))
	for {
		c, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.handleConn(c)
	}
}

func (s *server) auth(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	user := c.User()
	if name, ok := strings.CutPrefix(user, "agent-"); ok {
		keys, known := s.agentKeys[name]
		if !known {
			return nil, fmt.Errorf("unknown agent %q", name)
		}
		if !keyutil.KeyAllowed(key, keys) {
			return nil, fmt.Errorf("key not authorized for agent %q", name)
		}
		log.Printf("agent %q authenticated from %s", name, c.RemoteAddr())
		return &ssh.Permissions{Extensions: map[string]string{"role": "agent", "name": name}}, nil
	}
	adm, known := s.admins[user]
	if !known {
		return nil, fmt.Errorf("unknown admin %q", user)
	}
	if !keyutil.KeyAllowed(key, adm.keys) {
		return nil, fmt.Errorf("key not authorized for admin %q", user)
	}
	log.Printf("admin %q authenticated from %s", user, c.RemoteAddr())
	return &ssh.Permissions{Extensions: map[string]string{"role": "admin", "name": user}}, nil
}

func (s *server) handleConn(c net.Conn) {
	conn, chans, reqs, err := ssh.NewServerConn(c, s.sshCfg)
	if err != nil {
		log.Printf("handshake from %s failed: %v", c.RemoteAddr(), err)
		c.Close()
		return
	}
	switch conn.Permissions.Extensions["role"] {
	case "agent":
		s.handleAgent(conn, conn.Permissions.Extensions["name"], chans, reqs)
	case "admin":
		s.handleAdmin(conn, conn.Permissions.Extensions["name"], chans, reqs)
	default:
		conn.Close()
	}
}

func (s *server) handleAgent(conn *ssh.ServerConn, name string, chans <-chan ssh.NewChannel, reqs <-chan *ssh.Request) {
	ac := &agentConn{name: name, conn: conn, ports: map[uint32]string{}}
	s.register(ac)
	defer s.unregister(ac)

	go func() {
		for ch := range chans {
			ch.Reject(ssh.UnknownChannelType, "agents do not open channels")
		}
	}()
	go sshx.Keepalive(conn, s.keepalive)

	for req := range reqs {
		switch req.Type {
		case "tcpip-forward", "cancel-tcpip-forward":
			var p tcpipForwardRequest
			if err := ssh.Unmarshal(req.Payload, &p); err != nil {
				req.Reply(false, nil)
				continue
			}
			ac.mu.Lock()
			if req.Type == "tcpip-forward" {
				ac.ports[p.Port] = p.Addr
			} else {
				delete(ac.ports, p.Port)
			}
			ac.mu.Unlock()
			log.Printf("agent %q %s %s:%d", name, req.Type, p.Addr, p.Port)
			req.Reply(true, nil)
		default:
			req.Reply(false, nil)
		}
	}
}

func (s *server) handleAdmin(conn *ssh.ServerConn, name string, chans <-chan ssh.NewChannel, reqs <-chan *ssh.Request) {
	go ssh.DiscardRequests(reqs)
	adm := s.admins[name]
	for newCh := range chans {
		if newCh.ChannelType() != "direct-tcpip" {
			newCh.Reject(ssh.UnknownChannelType, "only direct-tcpip is supported")
			continue
		}
		var d directTCPIPData
		if err := ssh.Unmarshal(newCh.ExtraData(), &d); err != nil {
			newCh.Reject(ssh.ConnectionFailed, "bad direct-tcpip payload")
			continue
		}
		if !adm.allowed(d.Host) {
			log.Printf("admin %q denied target %q", name, d.Host)
			newCh.Reject(ssh.Prohibited, "not allowed to reach "+d.Host)
			continue
		}
		ac := s.lookup(d.Host)
		if ac == nil {
			newCh.Reject(ssh.ConnectionFailed, "no agent registered as "+d.Host)
			continue
		}
		ac.mu.Lock()
		addr, ok := ac.ports[d.Port]
		ac.mu.Unlock()
		if !ok {
			newCh.Reject(ssh.ConnectionFailed, fmt.Sprintf("agent %q has no listener on port %d", d.Host, d.Port))
			continue
		}
		ohost, oport := originOf(conn.RemoteAddr())
		agentCh, agentReqs, err := ac.conn.OpenChannel("forwarded-tcpip", ssh.Marshal(&forwardedTCPPayload{
			Addr:       addr,
			Port:       d.Port,
			OriginAddr: ohost,
			OriginPort: oport,
		}))
		if err != nil {
			log.Printf("open channel to agent %q failed: %v", d.Host, err)
			newCh.Reject(ssh.ConnectionFailed, "agent channel open failed: "+err.Error())
			continue
		}
		go ssh.DiscardRequests(agentReqs)
		adminCh, adminReqs, err := newCh.Accept()
		if err != nil {
			agentCh.Close()
			continue
		}
		go ssh.DiscardRequests(adminReqs)
		log.Printf("admin %q -> agent %q port %d", name, d.Host, d.Port)
		go sshx.Bridge(adminCh, agentCh)
	}
}

func (s *server) register(ac *agentConn) {
	s.mu.Lock()
	if old, ok := s.agents[ac.name]; ok && old != ac {
		log.Printf("agent %q re-registered, closing previous connection", ac.name)
		old.conn.Close()
	}
	s.agents[ac.name] = ac
	s.mu.Unlock()
}

func (s *server) unregister(ac *agentConn) {
	s.mu.Lock()
	if cur, ok := s.agents[ac.name]; ok && cur == ac {
		delete(s.agents, ac.name)
	}
	s.mu.Unlock()
}

func (s *server) lookup(name string) *agentConn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.agents[name]
}

func originOf(addr net.Addr) (string, uint32) {
	host, portStr, err := net.SplitHostPort(addr.String())
	if err != nil {
		return "127.0.0.1", 1
	}
	if net.ParseIP(host) == nil {
		host = "127.0.0.1"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		port = 1
	}
	return host, uint32(port)
}
