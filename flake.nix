{
  description = "Skribbl — TranscripTonic meeting notes wrapper + local LLM handoff";

  inputs.nixpkgs.url = "nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      system = "aarch64-darwin";
      pkgs   = nixpkgs.legacyPackages.${system};
    in
    {
      # ── binary package ───────────────────────────────────────────────────────
      packages.${system}.default = pkgs.buildGoModule {
        pname   = "skribbl";
        version = "0.1.0";
        src     = ./.;
        # vendor/ is committed — no network needed in the nix sandbox
        vendorHash = null;
      };

      # ── nix-darwin module ────────────────────────────────────────────────────
      darwinModules.default = { config, lib, pkgs, ... }:
        import ./module.nix {
          inherit config lib pkgs;
          skribbl-pkg = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
        };
    };
}
