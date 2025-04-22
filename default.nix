{ lib
, buildGoModule
, fetchFromGitHub
}: 

buildGoModule rec {
  pname = "ctrl";
  version = "0.3.0";

  src = fetchFromGitHub {
    owner = "TaKO8Ki";
    repo = "ctrl";
    rev = "v${version}";
    sha256 = "sha256-K/HlHI8tqAu4JhCmKOaAn2gDNgkV0yHDgaKHQF+wQkE=";
  };

  vendorSha256 = "sha256-Hxl8qz1UXHB2qHKcNYI8YJHJuRBtDEHXKUgbqf9Jqtc=";

  meta = with lib; {
    description = "a c2 system";
    homepage = "https://github.com/postmannen/ctrl";
    license = licenses.agpl;
    maintainers = [ postmannen ];
    mainProgram = "ctrl";
  };
} 