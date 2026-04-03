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
      in
      {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gtk4
            gtk4-layer-shell
            pkg-config
          ];

          shellHook = ''
            export CGO_ENABLED=1
            export GOFLAGS="${GOFLAGS:+$GOFLAGS }-tags=gtk4"
          '';
        };
      });
}
