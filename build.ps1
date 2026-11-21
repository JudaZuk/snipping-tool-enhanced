# SPDX-License-Identifier: Apache-2.0
# Builds SnippingToolEnhanced.exe for x64 and ARM64 into .\dist
#   .\build.ps1                 test + build x64 and ARM64
#   .\build.ps1 -Arch amd64     x64 only (skips the ARM64 build)
#   .\build.ps1 -Run            build and start the x64 app
param([string]$Version = "1.4.0", [string[]]$Arch = @("amd64", "arm64"), [switch]$Run, [switch]$SkipTests)
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot
if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw "Go is not installed: winget install GoLang.Go" }
if (-not (Test-Path go.sum)) { go mod tidy; if ($LASTEXITCODE) { throw "go mod tidy failed" } }
# App icon, version info and manifest (cmd/snippingtoolenhanced/*.syso)
Write-Host "Generating resources..."
go run -tags genres tools/genres/main.go $Version
if ($LASTEXITCODE) { throw "resource generation failed" }
if (-not $SkipTests) {
  Write-Host "Running tests..."
  go test ./...; if ($LASTEXITCODE) { throw "tests failed" }
}
foreach ($a in $Arch) {
  $env:GOOS = "windows"; $env:GOARCH = $a
  New-Item -ItemType Directory -Force "dist\$a" | Out-Null
  # -v lists each package as it is compiled, so a long first build shows progress.
  # Packages that are already compiled (Go's build cache) are not listed.
  Write-Host "Building $a... (the first build for an architecture compiles Go's standard library and can take several minutes)"
  $t = [Diagnostics.Stopwatch]::StartNew()
  go build -v -trimpath -ldflags "-s -w -H windowsgui -X snippingtoolenhanced/internal/editor.Version=$Version" -o "dist\$a\SnippingToolEnhanced.exe" .\cmd\snippingtoolenhanced
  if ($LASTEXITCODE) { throw "build failed ($a)" }
  Write-Host ("Built dist\$a\SnippingToolEnhanced.exe in {0:N0} s" -f $t.Elapsed.TotalSeconds)
}
Remove-Item Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue
if ($Run) { Start-Process "dist\amd64\SnippingToolEnhanced.exe" }
