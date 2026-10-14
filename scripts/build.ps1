# Builds the frontend and aicw.exe on Windows. Usage: scripts\build.ps1 [version]
# Needs Go, Node.js and the WebView2 runtime (preinstalled on Windows 11).
param([string]$Version = "")
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")
if (-not $Version) { $Version = (git describe --tags --always 2>$null); if (-not $Version) { $Version = "0.1.0-dev" } }
Push-Location frontend; npm ci --silent; npm run build --silent; Pop-Location
New-Item -ItemType Directory -Force build\bin | Out-Null
go build -tags "desktop,production" -ldflags "-s -w -H windowsgui -X main.Version=$Version" -o build\bin\aicw.exe .
Write-Host "built build\bin\aicw.exe ($Version)"
