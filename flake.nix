{
  description = "way-island development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        way-island = pkgs.buildGoModule {
          pname = "way-island";
          version = "dev";
          src = ./.;
          nativeBuildInputs = with pkgs; [
            pkg-config
          ];
          buildInputs = with pkgs; [
            gtk4
            gtk4-layer-shell
          ];
          tags = [ "gtk4" ];
          vendorHash = "sha256-sYMfACCgaOi0M9MktRjFj2Qn+D1L1IFO2DLQLE0JAzs=";
          postInstall = ''
            install -Dm644 ${./packaging/systemd/user/way-island.service} \
              $out/share/systemd/user/way-island.service
          '';
        };
      in
      {
        packages.default = way-island;

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gtk4
            gtk4-layer-shell
            pkg-config
            xauth
            xorg-server
            wlrctl
          ];

          shellHook = ''
            export CGO_ENABLED=1
            export GOFLAGS="''${GOFLAGS:+$GOFLAGS }-tags=gtk4"
          '';
        };
      });
}
