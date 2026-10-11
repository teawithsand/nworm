{
  description = "nworm - single-port SSH rendezvous for hosts behind NAT";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f system);
      pkgsFor = system: import nixpkgs { inherit system; };

      # Dependencies are vendored in ./vendor, so no vendorHash is needed.
      mkNworm = pkgs: pkgs.buildGoModule {
        pname = "nworm";
        version = "0.1.0";
        src = self;
        vendorHash = null;
        subPackages = [ "." ];
        ldflags = [ "-s" "-w" ];
        meta = {
          description = "Single-port SSH rendezvous for hosts behind NAT";
          license = pkgs.lib.licenses.mit;
          mainProgram = "nworm";
        };
      };
    in
    {
      packages = forAllSystems (system:
        let pkgs = pkgsFor system; in {
          nworm = mkNworm pkgs;
          default = mkNworm pkgs;
        });

      devShells = forAllSystems (system:
        let pkgs = pkgsFor system; in {
          default = pkgs.mkShell {
            packages = [ pkgs.go pkgs.gopls pkgs.gotools pkgs.golangci-lint ];
          };
        });

      formatter = forAllSystems (system: (pkgsFor system).nixfmt-rfc-style);

      nixosModules.default = { config, lib, pkgs, ... }:
        let
          cfg = config.services.nworm;
          yaml = pkgs.formats.yaml { };
          relayConfig = yaml.generate "nworm-relay.yaml" cfg.relay.settings;
          agentConfig = yaml.generate "nworm-agent.yaml" cfg.agent.settings;
          # ":443" -> 443, "0.0.0.0:443" -> 443. IPv6 literals are not parsed here.
          portFromListen = listen: lib.toInt (lib.last (lib.splitString ":" listen));
        in
        {
          options.services.nworm = {
            relay = {
              enable = lib.mkEnableOption "the nworm relay (public VPS)";
              package = lib.mkOption {
                type = lib.types.package;
                default = self.packages.${pkgs.system}.nworm;
                defaultText = lib.literalExpression "nworm";
              };
              settings = lib.mkOption {
                type = yaml.type;
                default = { };
                description = "Contents of relay.yaml.";
              };
              user = lib.mkOption { type = lib.types.str; default = "nworm"; };
              openFirewall = lib.mkOption { type = lib.types.bool; default = false; };
            };

            agent = {
              enable = lib.mkEnableOption "the nworm agent (host behind NAT)";
              package = lib.mkOption {
                type = lib.types.package;
                default = self.packages.${pkgs.system}.nworm;
                defaultText = lib.literalExpression "nworm";
              };
              settings = lib.mkOption {
                type = yaml.type;
                default = { };
                description = "Contents of agent.yaml.";
              };
              user = lib.mkOption { type = lib.types.str; default = "nworm"; };
            };
          };

          config = lib.mkMerge [
            (lib.mkIf cfg.relay.enable {
              users.groups.${cfg.relay.user} = { };
              users.users.${cfg.relay.user} = {
                isSystemUser = true;
                group = cfg.relay.user;
                home = "/var/lib/nworm";
                createHome = true;
              };
              systemd.services.nworm-relay = {
                description = "nworm relay";
                wantedBy = [ "multi-user.target" ];
                after = [ "network-online.target" ];
                wants = [ "network-online.target" ];
                serviceConfig = {
                  ExecStart = "${cfg.relay.package}/bin/nworm relay -config ${relayConfig}";
                  User = cfg.relay.user;
                  Restart = "always";
                  RestartSec = 3;
                  StateDirectory = "nworm";
                  NoNewPrivileges = true;
                  PrivateTmp = true;
                  ProtectSystem = "full";
                  ProtectHome = "read-only";
                };
              };
              networking.firewall.allowedTCPPorts =
                lib.mkIf cfg.relay.openFirewall
                  (lib.optional (cfg.relay.settings ? listen) (portFromListen cfg.relay.settings.listen));
            })

            (lib.mkIf cfg.agent.enable {
              users.groups.${cfg.agent.user} = { };
              users.users.${cfg.agent.user} = {
                isSystemUser = true;
                group = cfg.agent.user;
                home = "/var/lib/nworm";
                createHome = true;
              };
              systemd.services.nworm-agent = {
                description = "nworm agent";
                wantedBy = [ "multi-user.target" ];
                after = [ "network-online.target" ];
                wants = [ "network-online.target" ];
                serviceConfig = {
                  ExecStart = "${cfg.agent.package}/bin/nworm agent -config ${agentConfig}";
                  User = cfg.agent.user;
                  Restart = "always";
                  RestartSec = 3;
                  StateDirectory = "nworm";
                  NoNewPrivileges = true;
                  PrivateTmp = true;
                  ProtectSystem = "full";
                  ProtectHome = "read-only";
                };
              };
            })
          ];
        };
    };
}
