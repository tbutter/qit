{
  description = "Lightweight Query Tool";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, utils }:
    utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "qit";
          version = "0.1.0";
          src = ./.;

          vendorHash = "sha256-/xjeTsyvFIy0aKvWCF3DDpejIOOLDQe8djQk9vPozFw=";

          subPackages = [ "." ];

          meta = with pkgs.lib; {
            description = "Lightweight Query Tool";
            homepage = "https://github.com/tbutter/qit";
            license = licenses.mit;
          };
        };

        apps.default = utils.lib.mkApp {
          drv = self.packages.${system}.default;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            gotools
          ];
        };
      });
}
