{
  description = "Ryoport fleet dashboard";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };
      ryoport = pkgs.buildGoModule {
        pname = "ryoport";
        version = "1.0.0";
        src = ./.;
        vendorHash = null;

        nativeBuildInputs = with pkgs; [
          pkg-config
          makeWrapper
        ];

        buildInputs = with pkgs; [
          wayland
          libxkbcommon
          libGL
          libglvnd
          vulkan-headers
          vulkan-loader
          xorg.libX11
          xorg.libXcursor
          xorg.libXfixes
          xorg.libXi
          xorg.libXrandr
          xorg.libxcb
        ];

        buildPhase = ''
          runHook preBuild
          make build
          runHook postBuild
        '';

        # buildGoModule's own check phase relies on helpers from the build
        # phase replaced above, so run the suite the way the Makefile does.
        checkPhase = ''
          runHook preCheck
          HOME="$TMPDIR" make test
          runHook postCheck
        '';

        installPhase = ''
          runHook preInstall
          make SKIP_BUILD=1 DESTDIR="$out" PREFIX= install
          mv "$out/bin/ryoport" "$out/bin/.ryoport-unwrapped"
          rm "$out/bin/ryovm" "$out/bin/ryossh" "$out/bin/ryovm-mon" "$out/bin/ryovm-fetch"
          for command in ryoport ryovm ryossh ryovm-mon ryovm-fetch; do
            makeWrapper "$out/bin/.ryoport-unwrapped" "$out/bin/$command" \
              --argv0 "$command" \
              --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.openssh pkgs.libsecret pkgs.xdg-utils pkgs.xdg-terminal-exec ]}
          done
          runHook postInstall
        '';

        meta = {
          description = "Fleet dashboard for machines, servers and consoles";
          homepage = "https://github.com/ryoku-dev/ryoport";
          license = pkgs.lib.licenses.gpl3Plus;
          mainProgram = "ryoport";
          platforms = [ "x86_64-linux" ];
        };
      };
    in {
      packages.${system}.default = ryoport;
      apps.${system}.default = {
        type = "app";
        program = "${ryoport}/bin/ryoport";
        meta = { inherit (ryoport.meta) description; };
      };
    };
}
