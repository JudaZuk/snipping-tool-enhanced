# SPDX-License-Identifier: Apache-2.0
# Builds SnippingToolEnhanced.exe for x64 and ARM64 into .\dist
#   .\build.ps1            test + build
#   .\build.ps1 -Run       build and start the x64 app
param([string]$Version = "1.3.0", [switch]$Run, [switch]$SkipTests)
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot
if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw "Go is not installed: winget install GoLang.Go" }
if (-not (Test-Path go.sum)) { go mod tidy; if ($LASTEXITCODE) { throw "go mod tidy failed" } }
# App icon, version info and manifest (cmd/snippingtoolenhanced/*.syso)
go run -tags genres tools/genres/main.go $Version
if ($LASTEXITCODE) { throw "resource generation failed" }
if (-not $SkipTests) { go test ./...; if ($LASTEXITCODE) { throw "tests failed" } }
foreach ($a in "amd64", "arm64") {
  $env:GOOS = "windows"; $env:GOARCH = $a
  New-Item -ItemType Directory -Force "dist\$a" | Out-Null
  go build -trimpath -ldflags "-s -w -H windowsgui -X snippingtoolenhanced/internal/editor.Version=$Version" -o "dist\$a\SnippingToolEnhanced.exe" .\cmd\snippingtoolenhanced
  if ($LASTEXITCODE) { throw "build failed ($a)" }
}
Remove-Item Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue
if ($Run) { Start-Process "dist\amd64\SnippingToolEnhanced.exe" }
