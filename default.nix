{ pkgs ? import
    (fetchTarball {
      name = "jpetrucciani-2026-07-24";
      url = "https://github.com/jpetrucciani/nix/archive/095c660863348be59cb3051cf117b9295df34806.tar.gz";
      sha256 = "0yps42zjzfq69g3rzyh8x36bii90hzcqhhskkiyac155nm99l9l5";
    })
    { }
}:
let
  name = "skribbl";

  tools = with pkgs; {
    cli = [
      jfmt
      nixup
    ];
    go = [
      go
      go-tools
      gopls
      gcc
    ];
    scripts = pkgs.lib.attrsets.attrValues scripts;
  };

  scripts = with pkgs; { };
  paths = pkgs.lib.flatten [ (builtins.attrValues tools) ];
  env = pkgs.buildEnv {
    inherit name paths; buildInputs = paths;
  };
in
(env.overrideAttrs (_: {
  inherit name;
  NIXUP = "0.0.10";
})) // { inherit scripts; }
