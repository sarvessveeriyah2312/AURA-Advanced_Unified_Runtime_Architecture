<#
.SYNOPSIS
Tread installer for Windows (ADR-0011 §7).

.DESCRIPTION
Installs the Tread SDK (or the Tread Runtime) for the current user. No
administrator rights are needed, and Go is not required.

  powershell -NoProfile -ExecutionPolicy Bypass -Command "irm https://github.com/sarvessveeriyah2312/Tread/releases/latest/download/install.ps1 | iex"
  powershell -NoProfile -ExecutionPolicy Bypass -File install.ps1 [-Runtime] [-Version 0.4.0] [-Prefix DIR] [-Archive FILE] [-Checksums FILE] [-NoModifyPath] [-Uninstall]

The archive is verified against SHA256SUMS before anything is installed.
Only files this script installed are replaced or removed; the install
directory must be empty or a previous Tread installation. The user PATH
gets one entry (never duplicated), and TREAD_HOME is set; both are
removed on uninstall. No other environment variable is changed.
#>
[CmdletBinding()]
param(
    [switch]$Runtime,
    [string]$Version = "",
    [string]$Prefix = (Join-Path $HOME ".tread"),
    [string]$Archive = "",
    [string]$Checksums = "",
    [switch]$NoModifyPath,
    [switch]$Uninstall
)

$ErrorActionPreference = "Stop"
# Windows PowerShell 5.1 may not enable TLS 1.2, which GitHub requires.
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
$Repo = "https://github.com/sarvessveeriyah2312/Tread"
$ManifestName = ".tread-install-manifest"

function Fail([string]$msg) {
    Write-Host "tread install: error: $msg" -ForegroundColor Red
    exit 1
}

$Prefix = [System.IO.Path]::GetFullPath($Prefix).TrimEnd('\')
$Bin = Join-Path $Prefix "bin"
$Manifest = Join-Path $Prefix $ManifestName

function Get-UserPath {
    $p = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($null -eq $p) { return "" }
    return $p
}

# PATH entries compared without trailing backslashes, case-insensitively.
function Test-SamePath([string]$a, [string]$b) {
    return $a.TrimEnd('\') -ieq $b.TrimEnd('\')
}

function Remove-PathEntry {
    $parts = (Get-UserPath) -split ';' | Where-Object { $_ -ne "" -and -not (Test-SamePath $_ $Bin) }
    [Environment]::SetEnvironmentVariable("Path", ($parts -join ';'), "User")
}

function Add-PathEntry {
    $parts = @((Get-UserPath) -split ';' | Where-Object { $_ -ne "" })
    if (-not ($parts | Where-Object { Test-SamePath $_ $Bin })) {
        $parts += $Bin
        [Environment]::SetEnvironmentVariable("Path", ($parts -join ';'), "User")
    }
    [Environment]::SetEnvironmentVariable("TREAD_HOME", $Prefix, "User")
}

function Remove-Installed {
    if (-not (Test-Path -LiteralPath $Manifest)) { return }
    foreach ($rel in Get-Content -LiteralPath $Manifest) {
        if ($rel -eq "" -or $rel.Contains("..") -or [System.IO.Path]::IsPathRooted($rel)) { continue }
        $f = Join-Path $Prefix $rel
        if (Test-Path -LiteralPath $f) { Remove-Item -LiteralPath $f -Force }
    }
    Remove-Item -LiteralPath $Manifest -Force
    foreach ($d in @($Bin, (Join-Path $Prefix "docs"), $Prefix)) {
        if ((Test-Path -LiteralPath $d) -and -not (Get-ChildItem -LiteralPath $d -Force)) {
            Remove-Item -LiteralPath $d -Force
        }
    }
}

if ($Uninstall) {
    if (-not (Test-Path -LiteralPath $Manifest)) { Fail "no installation made by this script in $Prefix" }
    Remove-Installed
    Remove-PathEntry
    if (Test-SamePath ([Environment]::GetEnvironmentVariable("TREAD_HOME", "User")) $Prefix) {
        [Environment]::SetEnvironmentVariable("TREAD_HOME", $null, "User")
    }
    Write-Host "Removed Tread from $Prefix. Open a new terminal for the PATH change to take effect."
    exit 0
}

# Architecture validation: release builds exist for x64 only on Windows.
$arch = $env:PROCESSOR_ARCHITEW6432
if (-not $arch) { $arch = $env:PROCESSOR_ARCHITECTURE }
if ($arch -ne "AMD64" -and $arch -ne "ARM64") { Fail "unsupported CPU architecture $arch; Tread for Windows needs x64" }
if ($arch -eq "ARM64") { Write-Host "note: installing the x64 build, which runs under emulation on Windows on ARM." }
$Platform = "windows-x64"
$Edition = if ($Runtime) { "runtime" } else { "sdk" }

$Tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("tread-install-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $Tmp | Out-Null
try {
    if ($Archive -eq "") {
        $ProgressPreference = "SilentlyContinue"
        if ($Version -eq "") {
            $resp = Invoke-WebRequest -Uri "$Repo/releases/latest" -UseBasicParsing -MaximumRedirection 5
            $final = $resp.BaseResponse.ResponseUri
            if ($null -eq $final) { $final = $resp.BaseResponse.RequestMessage.RequestUri }
            $Version = ($final.AbsoluteUri -split '/v')[-1]
            if ($Version -notmatch '^\d+\.\d+\.\d+') { Fail "cannot determine the latest Tread version; use -Version" }
        }
        $Version = $Version.TrimStart('v')
        $name = "tread-$Edition-$Version-$Platform.zip"
        Write-Host "Downloading Tread $Version ($Edition, $Platform)..."
        Invoke-WebRequest -Uri "$Repo/releases/download/v$Version/$name" -OutFile (Join-Path $Tmp $name) -UseBasicParsing
        Invoke-WebRequest -Uri "$Repo/releases/download/v$Version/SHA256SUMS" -OutFile (Join-Path $Tmp "SHA256SUMS") -UseBasicParsing
        $Archive = Join-Path $Tmp $name
        $Checksums = Join-Path $Tmp "SHA256SUMS"
    } else {
        if (-not (Test-Path -LiteralPath $Archive)) { Fail "archive $Archive does not exist" }
        $Archive = (Resolve-Path -LiteralPath $Archive).Path
        if ($Checksums -eq "") { $Checksums = Join-Path (Split-Path $Archive) "SHA256SUMS" }
        if (-not (Test-Path -LiteralPath $Checksums)) { Fail "no checksum file $Checksums; pass -Checksums" }
    }

    # Verify before extracting anything.
    $base = Split-Path $Archive -Leaf
    $want = $null
    foreach ($line in Get-Content -LiteralPath $Checksums) {
        $f = $line -split '\s+', 2
        if ($f.Count -eq 2 -and $f[1].TrimStart('*') -eq $base) { $want = $f[0].ToLower(); break }
    }
    if (-not $want) { Fail "$base is not listed in $(Split-Path $Checksums -Leaf)" }
    $got = (Get-FileHash -LiteralPath $Archive -Algorithm SHA256).Hash.ToLower()
    if ($got -ne $want) { Fail "checksum mismatch for ${base}: expected $want, got $got" }
    Write-Host "Verified $base (SHA-256 $got)."

    if ($base -notlike "*-$Platform.zip") { Fail "$base is not for this computer ($Platform)" }
    if ($base -like "tread-runtime-*") { $Edition = "runtime" } else { $Edition = "sdk" }

    $x = Join-Path $Tmp "x"
    Expand-Archive -LiteralPath $Archive -DestinationPath $x
    $top = Join-Path $x ($base -replace '\.zip$', '')
    if (-not (Test-Path -LiteralPath (Join-Path $top "bin\tread.exe")) -or -not (Test-Path -LiteralPath (Join-Path $top "release"))) {
        Fail "$base does not contain a Tread installation"
    }

    # The target must be empty, absent, or a previous installation.
    if ((Test-Path -LiteralPath $Prefix) -and -not (Test-Path -LiteralPath $Manifest)) {
        if (Get-ChildItem -LiteralPath $Prefix -Force) {
            Fail "$Prefix exists and is not a Tread installation made by this script; choose another -Prefix"
        }
    }
    $previous = ""
    $rel = Join-Path $Prefix "release"
    if (Test-Path -LiteralPath $rel) {
        $m = Select-String -LiteralPath $rel -Pattern '^TREAD_VERSION="(.*)"$'
        if ($m) { $previous = $m.Matches[0].Groups[1].Value }
    }
    Remove-Installed

    # Relative paths via Resolve-Path, which works in Windows PowerShell 5.1
    # and is not confused by short (8.3) temporary paths.
    Push-Location -LiteralPath $top
    try {
        $files = @(Get-ChildItem -Recurse -File | Resolve-Path -Relative | ForEach-Object { $_ -replace '^\.\\', '' } | Sort-Object)
    } finally {
        Pop-Location
    }
    foreach ($f in $files) {
        $dest = Join-Path $Prefix $f
        New-Item -ItemType Directory -Force -Path (Split-Path $dest) | Out-Null
        Copy-Item -LiteralPath (Join-Path $top $f) -Destination $dest -Force
    }
    Set-Content -LiteralPath $Manifest -Value $files -Encoding ASCII

    & (Join-Path $Bin "tread.exe") --version | Out-Null
    if ($LASTEXITCODE -ne 0) { Fail "the installed tread does not run; please report this" }

    if (-not $NoModifyPath) {
        Add-PathEntry
        # Also update this session, so tread works right away.
        if (-not (($env:Path -split ';') | Where-Object { Test-SamePath $_ $Bin })) { $env:Path = "$env:Path;$Bin" }
        $env:TREAD_HOME = $Prefix
    }

    $installed = (Select-String -LiteralPath $rel -Pattern '^TREAD_VERSION="(.*)"$').Matches[0].Groups[1].Value
    $label = if ($Edition -eq "runtime") { "Tread Runtime" } else { "Tread SDK" }
    if ($previous -ne "" -and $previous -ne $installed) {
        Write-Host "Upgraded $label $previous -> $installed in $Prefix."
    } else {
        Write-Host "Installed $label $installed in $Prefix."
    }
    if ($NoModifyPath) {
        Write-Host "Add $Bin to PATH, then check with: tread doctor"
    } else {
        Write-Host "Open a new terminal, then check with: tread doctor"
    }
} finally {
    Remove-Item -LiteralPath $Tmp -Recurse -Force -ErrorAction SilentlyContinue
}
