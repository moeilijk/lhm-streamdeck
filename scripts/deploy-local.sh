#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
plugin_dir="$root_dir/com.moeilijk.lhm.sdPlugin"

win_user="${WIN_USER:-}"
if [[ -z "$win_user" ]]; then
  win_user="$(powershell.exe -NoProfile -Command '$env:UserName' 2>/dev/null | tr -d '\r')"
fi
if [[ -z "$win_user" ]]; then
  echo "Unable to resolve Windows username. Set WIN_USER and retry." >&2
  exit 1
fi

win_plugin_dir="/mnt/c/Users/$win_user/AppData/Roaming/Elgato/StreamDeck/Plugins/com.moeilijk.lhm.sdPlugin"
if [[ "$win_plugin_dir" != /mnt/c/Users/*/AppData/Roaming/Elgato/StreamDeck/Plugins/com.moeilijk.lhm.sdPlugin ]]; then
  echo "Unsafe target path: $win_plugin_dir" >&2
  exit 1
fi

echo "kill: Stream Deck + plugin processes"
powershell.exe -NoProfile -Command "Get-Process StreamDeck,lhm,lhm-bridge -ErrorAction SilentlyContinue | Stop-Process -Force" >/dev/null 2>&1 || true

echo "clean: $win_plugin_dir"
for _ in 1 2 3 4 5; do
  rm -rf "$win_plugin_dir" >/dev/null 2>&1 || true
  if [[ ! -e "$win_plugin_dir" ]]; then
    break
  fi
  sleep 1
done
mkdir -p "$win_plugin_dir"

echo "copy: $plugin_dir -> $win_plugin_dir"
rsync -a --delete "$plugin_dir/" "$win_plugin_dir/"

echo "migrate: settings title state in Stream Deck profiles"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$(wslpath -w "$root_dir/scripts/fix-settings-title-alignment.ps1")" -WinUser "$win_user"

echo "start: Stream Deck"
powershell.exe -NoProfile -Command "\$p1 = \"\$env:ProgramFiles\\Elgato\\StreamDeck\\StreamDeck.exe\"; \$p2 = \"\$env:ProgramFiles(x86)\\Elgato\\StreamDeck\\StreamDeck.exe\"; if (Test-Path \$p1) { Start-Process -FilePath \$p1; exit 0 }; if (Test-Path \$p2) { Start-Process -FilePath \$p2; exit 0 }; Write-Error \"StreamDeck.exe not found\"; exit 1" >/dev/null
