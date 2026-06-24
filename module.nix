{ config, lib, pkgs, sinch-meetings-pkg }:
let
  cfg = config.services.sinch-meetings;
  home = "/Users/${cfg.user}";
in
{
  options.services.sinch-meetings = {
    enable = lib.mkEnableOption "Sinch meetings webhook + KB processor";

    user = lib.mkOption {
      type    = lib.types.str;
      description = "User account to run the service as";
    };

    secretsDir = lib.mkOption {
      type    = lib.types.str;
      default = "${home}/.config/sinch/meetings";
      description = "Directory containing the .env file — lives outside the nix store";
    };

    vaultPath = lib.mkOption {
      type    = lib.types.str;
      default = "${home}/vaults/meetings";
      description = "Path to the Obsidian meetings vault";
    };

    port = lib.mkOption {
      type    = lib.types.port;
      default = 5050;
      description = "Port for the webhook HTTP server";
    };

    logDir = lib.mkOption {
      type    = lib.types.str;
      default = "${home}/Library/Logs/sinch-meetings";
      description = "Directory for service logs";
    };
  };

  config = lib.mkIf cfg.enable {

    # ── webhook + processor ──────────────────────────────────────────────────
    launchd.user.agents."com.sinch.meetings" = {
      serviceConfig = {
        ProgramArguments = [ "${sinch-meetings-pkg}/bin/meetings" ];
        # godotenv loads .env from WorkingDirectory — secrets never enter the nix store
        WorkingDirectory  = cfg.secretsDir;
        KeepAlive         = true;
        RunAtLoad         = true;
        StandardOutPath   = "${cfg.logDir}/meetings.log";
        StandardErrorPath = "${cfg.logDir}/meetings.log";
        ThrottleInterval  = 5;
      };
    };

    # ── vault git autocommit (hourly) ────────────────────────────────────────
    launchd.user.agents."com.sinch.meetings-vault-commit" = {
      serviceConfig = {
        ProgramArguments = [
          "${pkgs.bash}/bin/bash" "-c"
          ''
            cd ${cfg.vaultPath} || exit 0
            ${pkgs.git}/bin/git add -A
            ${pkgs.git}/bin/git diff --cached --quiet && exit 0
            ${pkgs.git}/bin/git \
              -c user.name="gjallar" \
              -c user.email="fisherrjd@gmail.com" \
              commit -m "auto: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
          ''
        ];
        StartInterval = 3600;
        RunAtLoad     = false;
      };
    };

  };
}
