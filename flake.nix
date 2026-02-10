{
  description = "P2P-based distributed collaboration system for AI agents";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        version = if (self ? rev) then self.shortRev else "dev";

        agent-collab = pkgs.buildGoModule {
          pname = "agent-collab";
          inherit version;

          src = self;

          # Hash of go.sum dependencies (use lib.fakeHash to recalculate)
          vendorHash = "sha256-myTGzN21Xb4hNHemeFwEjJaz7f4I6byk61PS+BTbcbo=";

          ldflags = [
            "-s" "-w"
            "-X main.version=${version}"
            "-X main.commit=${self.rev or "unknown"}"
            "-X main.date=1970-01-01T00:00:00Z"
            "-X main.builtBy=nix"
          ];

          subPackages = [ "src" ];

          postInstall = ''
            mv $out/bin/src $out/bin/agent-collab
          '';

          # Skip tests during build (they require network)
          doCheck = false;

          meta = with pkgs.lib; {
            description = "P2P-based distributed collaboration system for AI agents";
            homepage = "https://github.com/yourusername/agent-collab";
            license = licenses.mit;
            maintainers = [];
            mainProgram = "agent-collab";
          };
        };
      in
      {
        packages = {
          default = agent-collab;
          agent-collab = agent-collab;
        };

        apps = {
          default = {
            type = "app";
            program = "${agent-collab}/bin/agent-collab";
          };
          agent-collab = {
            type = "app";
            program = "${agent-collab}/bin/agent-collab";
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go_1_22
            gopls
            gotools
            go-tools
            goreleaser
            golangci-lint
          ];

          shellHook = ''
            echo "agent-collab development environment"
            echo ""
            echo "Available commands:"
            echo "  go build ./cmd/agent-collab  - Build the binary"
            echo "  go test ./...                - Run tests"
            echo "  goreleaser check             - Validate goreleaser config"
            echo "  goreleaser build --snapshot  - Test build without releasing"
          '';
        };
      }
    ) // {
      # NixOS module
      nixosModules.default = { config, lib, pkgs, ... }:
        with lib;
        let
          cfg = config.services.agent-collab;
        in
        {
          options.services.agent-collab = {
            enable = mkEnableOption "agent-collab daemon";

            package = mkOption {
              type = types.package;
              default = self.packages.${pkgs.system}.default;
              description = "The agent-collab package to use.";
            };

            user = mkOption {
              type = types.str;
              default = "agent-collab";
              description = "User account under which agent-collab runs.";
            };

            group = mkOption {
              type = types.str;
              default = "agent-collab";
              description = "Group under which agent-collab runs.";
            };

            dataDir = mkOption {
              type = types.path;
              default = "/var/lib/agent-collab";
              description = "Directory for agent-collab data.";
            };
          };

          config = mkIf cfg.enable {
            users.users.${cfg.user} = {
              isSystemUser = true;
              group = cfg.group;
              home = cfg.dataDir;
              createHome = true;
            };

            users.groups.${cfg.group} = {};

            systemd.services.agent-collab = {
              description = "agent-collab daemon";
              wantedBy = [ "multi-user.target" ];
              after = [ "network.target" ];

              serviceConfig = {
                Type = "simple";
                User = cfg.user;
                Group = cfg.group;
                ExecStart = "${cfg.package}/bin/agent-collab daemon run";
                Restart = "on-failure";
                RestartSec = "5s";

                # Hardening
                NoNewPrivileges = true;
                ProtectSystem = "strict";
                ProtectHome = true;
                PrivateTmp = true;
                ReadWritePaths = [ cfg.dataDir ];
              };

              environment = {
                HOME = cfg.dataDir;
              };
            };
          };
        };

      # Home-manager module
      homeManagerModules.default = { config, lib, pkgs, ... }:
        with lib;
        let
          cfg = config.programs.agent-collab;
        in
        {
          options.programs.agent-collab = {
            enable = mkEnableOption "agent-collab";

            package = mkOption {
              type = types.package;
              default = self.packages.${pkgs.system}.default;
              description = "The agent-collab package to use.";
            };

            enableDaemon = mkOption {
              type = types.bool;
              default = false;
              description = "Whether to enable the daemon service.";
            };
          };

          config = mkIf cfg.enable {
            home.packages = [ cfg.package ];

            systemd.user.services.agent-collab = mkIf cfg.enableDaemon {
              Unit = {
                Description = "agent-collab daemon";
                After = [ "network.target" ];
              };

              Service = {
                Type = "simple";
                ExecStart = "${cfg.package}/bin/agent-collab daemon run";
                Restart = "on-failure";
                RestartSec = "5s";
              };

              Install = {
                WantedBy = [ "default.target" ];
              };
            };
          };
        };
    };
}
