# nworm — step by step

nworm provides SSH access to machines behind NAT through a **single public port** on a VPS.
There are three roles and one binary:

| Role | Where | Command |
|------|-------|---------|
| relay | public VPS | `nworm relay --config relay.yaml` |
| agent | host behind NAT | `nworm agent --config agent.yaml` |
| connect | your machine | `nworm connect <name>` |

The relay **never** connects to the target. The agent dials out to the relay and registers
a name; you connect to the relay and ask for that name. The relay joins both connections.
SSH stays end-to-end between you and the target host.

---

## 0. Before you start

- A VPS with a public IP and one open port (443 or 22).
- A host behind NAT (Linux with `sshd`; Windows with RDP — see section 7).
- Your computer.
- The `nworm` binary on all three machines (see section 8 NixOS and section 9 Docker).

## 1. Keys

Generate a separate nworm key on **each** machine:

```bash
# on the VPS (relay)
nworm keygen -f /etc/nworm/host.key -comment nworm-relay > /etc/nworm/host.key.pub

# on the host behind NAT (agent)
nworm keygen -f /etc/nworm/agent.key -comment agent@home

# on your computer (admin)
nworm keygen -f ~/.config/nworm/admin.key -comment admin@laptop
```

`nworm keygen` writes the private key (0600) and prints the public key. Paste the public
keys into the relay config. **Private keys never leave their machine.**

## 2. Relay on the VPS

Create `/etc/nworm/relay.yaml`:

```yaml
listen: ":443"
host_key: /etc/nworm/host.key
keepalive: 30s

agents:
  home:
    keys:
      - "ssh-ed25519 AAAA... agent@home"

admins:
  admin:
    keys:
      - "ssh-ed25519 AAAA... admin@laptop"
    allow: [home]
```

Run it:

```bash
nworm relay --config /etc/nworm/relay.yaml
```

At startup you will see the host key fingerprint, e.g.
`relay listening on :443 (host key SHA256:...)`. Open port 443 in the firewall.

### 2.1 systemd (manual)

```ini
# /etc/systemd/system/nworm-relay.service
[Unit]
Description=nworm relay
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/nworm relay --config /etc/nworm/relay.yaml
Restart=always
RestartSec=3
User=root
NoNewPrivileges=yes
ProtectSystem=full
PrivateTmp=yes

[Install]
WantedBy=multi-user.target
```

On NixOS do not write this by hand — see section 8.

### 2.2 known_hosts (trusting the relay)

On the agent and the admin, trust the relay's host key. The entry for port 443 is:

```
[relay.example.com]:443 ssh-ed25519 AAAA... nworm-relay
```

Save it as `/etc/nworm/known_hosts` (agent) and `~/.config/nworm/known_hosts` (admin).
Without it the connection is rejected — this protects against relay impersonation.

## 3. Agent on the host behind NAT

`/etc/nworm/agent.yaml`:

```yaml
name: home
identity: /etc/nworm/agent.key
relays:
  - "relay1.example.com:443"
remote: "127.0.0.1:22"    # what you register on the relay (nothing is bound)
local:  "127.0.0.1:22"    # where the agent dials on this host
known_hosts: /etc/nworm/known_hosts
keepalive: 30s
```

Run it:

```bash
nworm agent --config /etc/nworm/agent.yaml
```

The relay log will show `agent "home" authenticated` and
`agent "home" tcpip-forward 127.0.0.1:22`.

### 3.1 systemd (manual)

```ini
# /etc/systemd/system/nworm-agent.service
[Unit]
Description=nworm agent
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/nworm agent --config /etc/nworm/agent.yaml
Restart=always
RestartSec=3
User=root
NoNewPrivileges=yes

[Install]
WantedBy=multi-user.target
```

## 4. Admin (your computer)

`~/.config/nworm/connect.yaml`:

```yaml
admin_user: admin
identity: ~/.config/nworm/admin.key
known_hosts: ~/.config/nworm/known_hosts
relays:
  - "relay1.example.com:443"
```

`~/.ssh/config`:

```
Host home
  ProxyCommand nworm connect home
  User your-user-on-the-host
  HostKeyAlias home
  StrictHostKeyChecking yes
  IdentityFile ~/.ssh/id_ed25519
```

Now:

```bash
ssh home
```

What happens: `ssh` runs `nworm connect home`, which logs in to the relay with your admin
key; the relay joins a channel with agent `home`, the agent dials its `127.0.0.1:22`, and
`ssh` authenticates to the target's sshd with your normal key.

## 5. Multiple hosts

For each host add:

1. relay: `agents.office.keys = ["ssh-ed25519 ... agent@office"]`
2. agent on office: `name: office`
3. admin: `Host office` + `ProxyCommand nworm connect office`

One port on the VPS serves every name. Zero per-host ports.

## 6. Failover across multiple VPS

List several relays in `agent.yaml` — the agent registers on all of them:

```yaml
relays:
  - "relay1.example.com:443"
  - "relay2.example.com:443"
  - "relay3.example.com:443"
```

List them in `connect.yaml` too — `nworm connect` tries them in order.

## 7. RDP / Windows

On Windows run the agent as a service (NSSM or WinSW) with:

```yaml
name: home-rdp
identity: C:\nworm\agent.key
relays: ["relay1.example.com:443"]
remote: "127.0.0.1:3389"
local:  "127.0.0.1:3389"
known_hosts: C:\nworm\known_hosts
```

On the admin expose the stream locally and point RDP at `localhost:13389`:

```bash
nworm connect --config connect.yaml --listen 127.0.0.1:13389 home-rdp
```

## 8. NixOS (flake)

`flake.nix` provides the package, a NixOS module, and a devShell.

### 8.1 Package install

```bash
nix build            # ./result/bin/nworm
nix run . -- version
```

### 8.2 NixOS module — relay (VPS)

In your NixOS configuration:

```nix
{
  inputs.nworm.url = "github:teawithsand/nworm";
  # ...
  outputs = { self, nixpkgs, nworm, ... }: {
    nixosConfigurations.vps = nixpkgs.lib.nixosSystem {
      modules = [
        nworm.nixosModules.default
        ({ ... }: {
          services.nworm.relay = {
            enable = true;
            openFirewall = true;
            settings = {
              listen = ":443";
              host_key = "/etc/nworm/host.key";
              agents.home.keys = [ "ssh-ed25519 AAAA... agent@home" ];
              admins.admin.keys = [ "ssh-ed25519 AAAA... admin@laptop" ];
              admins.admin.allow = [ "home" ];
            };
          };
        })
      ];
    };
  };
}
```

Create the host key once by hand (do not put it in the Nix store):

```bash
sudo mkdir -p /etc/nworm
sudo nix run .#nworm -- keygen -f /etc/nworm/host.key -comment nworm-relay > /etc/nworm/host.key.pub
```

### 8.3 NixOS module — agent (host behind NAT)

```nix
services.nworm.agent = {
  enable = true;
  settings = {
    name = "home";
    identity = "/etc/nworm/agent.key";
    relays = [ "relay1.example.com:443" ];
    remote = "127.0.0.1:22";
    local = "127.0.0.1:22";
    known_hosts = "/etc/nworm/known_hosts";
  };
};
```

The module creates a system user, generates the YAML, and runs a systemd service
(`nworm-relay` / `nworm-agent`) with hardening (`NoNewPrivileges`, `ProtectSystem`).

## 9. Docker

Build the image:

```bash
docker build -t nworm .
```

Relay on the VPS:

```bash
docker run -d --name nworm-relay \
  --restart unless-stopped \
  -p 443:443 \
  -v /etc/nworm:/etc/nworm \
  nworm relay --config /etc/nworm/relay.yaml
```

Agent (on Linux, so it can reach the local sshd on 127.0.0.1:22):

```bash
docker run -d --name nworm-agent \
  --restart unless-stopped \
  --network host \
  -v /etc/nworm:/etc/nworm:ro \
  nworm agent --config /etc/nworm/agent.yaml
```

Note: inside a container `local: 127.0.0.1:22` is the container, not the host. Use
`--network host` or set `local` to the Docker gateway address.

## 10. Verification and troubleshooting

- The relay should log: `agent "home" authenticated` then `agent "home" tcpip-forward ...`.
- `nworm connect home` fails: check `known_hosts` and the admin key in `relay.yaml`.
- `no agent registered as home`: the agent is not running or the name in `agent.yaml` differs.
- `key not authorized for admin ...`: wrong public key in the `admins` section.
- `Port 443 already in use`: the relay binds `listen`; use another port or free 443.
- The session drops when the link breaks: SSH does not migrate a live session. Keep `tmux`
  on the target host and reconnect — you can switch relays.

## 11. Security

- Private keys: 0600, owner only, outside the Nix store.
- Pin the relay host key (`StrictHostKeyChecking yes`); `insecure: true` only in tests.
- The `allow` list is **default-deny**: empty/omitted means no access at all.
  `allow: ["home"]` permits specific names, and `allow: ["*"]` deliberately permits all.
- The agent's `local` is fixed — it cannot be turned into an arbitrary proxy; the relay
  routes only registered names.
- The relay does not see SSH content (end-to-end), but it does see metadata: who, when, to whom.

## 12. Environment variables

Every field can be set with an environment variable instead of the YAML file. Precedence:
**default < config file < environment < explicit flag**.

Naming: `NWORM_<COMMAND>_<FIELD>`, upper-cased (`host_key` becomes `HOST_KEY`).

```bash
NWORM_RELAY_LISTEN=":443" nworm relay --config /etc/nworm/relay.yaml
NWORM_AGENT_RELAYS="relay1:443,relay2:443" nworm agent --config /etc/nworm/agent.yaml
NWORM_CONNECT_ADMIN_USER="admin" NWORM_CONNECT_IDENTITY="$HOME/.config/nworm/admin.key" \
  nworm connect home
```

Types: scalars, booleans, and durations are native; lists accept CSV or a JSON array;
maps and nested structures accept JSON. An empty variable clears the field.

```bash
NWORM_CONNECT_DEST='{"home-rdp":3389}'
NWORM_RELAY_AGENTS='{"home":{"keys":["ssh-ed25519 AAAA... agent@home"]}}'
```

A ready-to-edit template is in [examples/relay.env](examples/relay.env),
[examples/agent.env](examples/agent.env), and [examples/connect.env](examples/connect.env).
Do not place private key material in environment files.
