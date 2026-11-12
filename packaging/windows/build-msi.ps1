<#
.SYNOPSIS
Builds the Tread SDK .msi from the Windows SDK archive (ADR-0011 §7).

.DESCRIPTION
Needs the WiX Toolset v5: dotnet tool install --global wix --version 5.0.2

  packaging\windows\build-msi.ps1 -Archive dist\tread-sdk-<version>-windows-x64.zip -OutDir dist
#>
param(
    [Parameter(Mandatory = $true)][string]$Archive,
    [Parameter(Mandatory = $true)][string]$OutDir
)
$ErrorActionPreference = "Stop"

$base = [System.IO.Path]::GetFileNameWithoutExtension($Archive)
if ($base -notmatch '^tread-sdk-(.+)-windows-x64$') { throw "$Archive is not a Windows SDK archive" }
$version = $Matches[1]
# MSI product versions are numeric (major.minor.build): drop a pre-release suffix.
$msiVersion = ($version -split '-')[0]

$work = Join-Path ([System.IO.Path]::GetTempPath()) ("tread-msi-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $work | Out-Null
try {
    Expand-Archive -LiteralPath $Archive -DestinationPath $work
    $src = Join-Path $work $base
    New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
    $out = Join-Path (Resolve-Path $OutDir) "$base.msi"
    wix build (Join-Path $PSScriptRoot "tread.wxs") -arch x64 -d "Version=$msiVersion" -d "SourceDir=$src" -o $out
    if ($LASTEXITCODE -ne 0) { throw "wix build failed" }
    Write-Host "build-msi: wrote $out"
} finally {
    Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue
}
