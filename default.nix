{ pkgs ? import
    (fetchTarball {
      name = "jpetrucciani-2026-06-24";
      url = "https://github.com/jpetrucciani/nix/archive/bdd13d0b1e4012fde1eda46fb524cbadf1ef35e8.tar.gz";
      sha256 = "1hd27q365vyajxy61fw15sgaibsxibyxf0s7jpv576qq8q01zbld";
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
