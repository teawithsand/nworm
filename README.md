# nworm

Single binary, one port, three modes: **relay** (VPS), **agent** (host behind NAT),
and **connect** (admin behind NAT). Authentication uses SSH keys everywhere. SSH stays
end-to-end between the admin and the target host — the relay only sees ciphertext.

```
  target (NAT) --out--> VPS:443 (relay) <--out-- admin (NAT)
```

The relay **never** connects to the target: the agent dials out and registers a name,
and the admin connects to the relay to ask for that name. **No per-host port is used** —
only the relay's single socket.

Full step-by-step guide: [docs/USAGE.md](docs/USAGE.md).

## Build

```bash
go build -o nworm .
# or
make build
# or hermetically
nix build          # ./result/bin/nworm
docker build -t nworm .
```

## Modes

```bash
nworm relay   --config /etc/nworm/relay.yaml
nworm agent   --config /etc/nworm/agent.yaml
nworm connect [flags] <name>
nworm keygen  -f id_ed25519 [-comment you@host]
```

Single-dash long flags such as `-config` still work (they are normalized to `--config`).

## Quick start

1. Keys (one per machine):

```bash
nworm keygen -f ~/.config/nworm/admin.key -comment admin@laptop
nworm keygen -f /etc/nworm/agent.key   -comment agent@home
```

2. Put the public keys into the relay config ([examples/relay.yaml](examples/relay.yaml)).

3. Pin the relay host key in known_hosts:

```
[relay.example.com]:443 ssh-ed25519 AAAA...
```

4. Start the relay and the agent, then on the admin:

```
Host home
  ProxyCommand nworm connect home
  User maciej
  HostKeyAlias home
  StrictHostKeyChecking yes
```

Now `ssh home` works.

## Configuration and environment

Precedence: **default < config file < environment < explicit flag**.

Environment variables use the prefix `NWORM_` plus the command segment and the field
name, upper-cased: `NWORM_<COMMAND>_<FIELD>`.

| Field | Environment variable |
|-------|----------------------|
| relay.listen | `NWORM_RELAY_LISTEN` |
| relay.host_key | `NWORM_RELAY_HOST_KEY` |
| relay.keepalive | `NWORM_RELAY_KEEPALIVE` |
| relay.agents | `NWORM_RELAY_AGENTS` (JSON) |
| relay.admins | `NWORM_RELAY_ADMINS` (JSON) |
| agent.name | `NWORM_AGENT_NAME` |
| agent.identity | `NWORM_AGENT_IDENTITY` |
| agent.relays | `NWORM_AGENT_RELAYS` (CSV or JSON) |
| agent.remote / local | `NWORM_AGENT_REMOTE` / `NWORM_AGENT_LOCAL` |
| agent.known_hosts | `NWORM_AGENT_KNOWN_HOSTS` |
| agent.insecure | `NWORM_AGENT_INSECURE` |
| connect.admin_user | `NWORM_CONNECT_ADMIN_USER` |
| connect.identity | `NWORM_CONNECT_IDENTITY` |
| connect.known_hosts | `NWORM_CONNECT_KNOWN_HOSTS` |
| connect.relays | `NWORM_CONNECT_RELAYS` (CSV or JSON) |
| connect.port | `NWORM_CONNECT_PORT` |
| connect.dest | `NWORM_CONNECT_DEST` (JSON) |

Lists accept CSV or a JSON array; maps and nested structures accept JSON. An empty
variable clears the field. See [examples/relay.env](examples/relay.env),
[examples/agent.env](examples/agent.env), and [examples/connect.env](examples/connect.env).

```bash
NWORM_RELAY_LISTEN=":443" nworm relay --config /etc/nworm/relay.yaml
NWORM_AGENT_RELAYS="relay1:443,relay2:443" nworm agent --config /etc/nworm/agent.yaml
NWORM_CONNECT_DEST='{"home-rdp":3389}' nworm connect home-rdp
```

Do not put private keys in an environment file; nworm only reads key paths from config.

## NixOS

The flake provides the package, a NixOS module, and a devShell:

```nix
{
  inputs.nworm.url = "github:teawithsand/nworm";
  # ...
  modules = [ nworm.nixosModules.default ({ ... }: {
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
  }) ];
}
```

More: [docs/USAGE.md](docs/USAGE.md).

## Docker

```bash
docker build -t nworm .
docker run -d --name nworm-relay --restart unless-stopped \
  -p 443:443 -v /etc/nworm:/etc/nworm \
  nworm relay --config /etc/nworm/relay.yaml
```

## CI

[.github/workflows/ci.yml](.github/workflows/ci.yml) runs on every pull request and push:

- `gofmt` + `go vet` (lint) and the English-only guard,
- `go test -race ./...` (config + in-process E2E),
- cross-build (linux/darwin/windows x amd64/arm64),
- `govulncheck` (security audit),
- `nix build .#nworm` (flake),
- `docker build` (GHA cache).

Dependabot tracks gomod, GitHub Actions, and Docker.

## Failover

The agent registers the same name on every relay in its list; `connect` tries the relays
in order. An SSH session does not migrate — reconnect and use `tmux` on the target host.

## RDP

Register a second name pointing at `127.0.0.1:3389`; on the admin run
`nworm connect --listen 127.0.0.1:13389 home-rdp` and point RDP at `localhost:13389`.

## Security

- Keep `host_key`, `identity`, and `known_hosts` outside the Nix store, in 0600/0644.
- The relay's `allow` list is default-deny (empty = no access); `allow: ["*"]` allows every name.
- Pin the relay host key (`StrictHostKeyChecking yes`); do not use `insecure: true` outside tests.
- The agent's `local` is fixed in its config, so it cannot be turned into an open proxy.

## Contributing

This repository is English-only: documentation, code comments, log and error messages,
and examples. CI enforces it.
