$manifest = "com.moeilijk.lhm.sdPlugin/manifest.json"
$content = Get-Content -Raw $manifest
$pattern = '"Version"\s*:\s*"(?<prefix>\d+\.\d+\.\d+\.)(?<build>\d+)"'

if ($content -notmatch $pattern) {
  Write-Error "Version not found or invalid format in manifest.json"
  exit 1
}

$prefix = $Matches["prefix"]
$build = [int]$Matches["build"] + 1
$newVersion = "$prefix$build"
$content = [regex]::Replace($content, $pattern, "\"Version\": \"$newVersion\"", 1)
Set-Content -Path $manifest -Value $content -Encoding utf8
Write-Output $newVersion
