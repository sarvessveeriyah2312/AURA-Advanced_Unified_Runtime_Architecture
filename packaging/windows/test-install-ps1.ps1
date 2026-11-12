<#
.SYNOPSIS
Tests scripts\install\install.ps1 end to end (ADR-0011 §7): user-level
install, PATH and TREAD_HOME, tread doctor, reinstall without duplicate
PATH entries, checksum and foreign-directory refusals, upgrade, runtime
edition, and uninstall. CI only.

  packaging\windows\test-install-ps1.ps1 -Dist dist -UpgradeDist dist-upgrade
#>
param(
    [Parameter(Mandatory = $true)][string]$Dist,
    [Parameter(Mandatory = $true)][string]$UpgradeDist
)
$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "test-lib.ps1")

$installer = Join-Path $PSScriptRoot "..\..\scripts\install\install.ps1"
function Version([string]$d) { (Select-String -Path "$d\BUILD-INFO.txt" -Pattern '^version:\s*(.+)$').Matches[0].Groups[1].Value }
$v1 = Version $Dist
$v2 = Version $UpgradeDist
$sdk = (Resolve-Path "$Dist\tread-sdk-$v1-windows-x64.zip").Path
$rt = (Resolve-Path "$Dist\tread-runtime-$v1-windows-x64.zip").Path
$sdk2 = (Resolve-Path "$UpgradeDist\tread-sdk-$v2-windows-x64.zip").Path
$prefix = Join-Path $HOME ".tread"
$bin = Join-Path $prefix "bin"
function Run-Installer { & pwsh -NoProfile -File $installer @args; return $LASTEXITCODE -eq 0 }

Check "fresh install" { Run-Installer -Archive $sdk }
Check "binary installed" { Test-Path "$bin\tread.exe" }
Check "user PATH has bin once" { (Count-PathEntry $bin) -eq 1 }
Check "TREAD_HOME set" { Test-SamePath ([Environment]::GetEnvironmentVariable("TREAD_HOME", "User")) $prefix }
Use-NewEnvironment
Check "version" { (tread --version) -eq "tread $v1" }
Check "doctor healthy" { $o = (tread doctor | Out-String); Write-Host $o; $o -match "Installation is healthy" }

Check "reinstall" { Run-Installer -Archive $sdk }
Check "PATH not duplicated" { (Count-PathEntry $bin) -eq 1 }

$bad = Join-Path $env:TEMP ("tread-bad-" + [guid]::NewGuid())
New-Item -ItemType Directory $bad | Out-Null
Copy-Item $sdk, "$Dist\SHA256SUMS" $bad
$f = Join-Path $bad ([IO.Path]::GetFileName($sdk))
$bytes = [IO.File]::ReadAllBytes($f); $bytes[100] = $bytes[100] -bxor 0xFF; [IO.File]::WriteAllBytes($f, $bytes)
Check "tampered archive rejected" { -not (Run-Installer -Archive $f -Prefix (Join-Path $bad "inst") -NoModifyPath) }
Check "nothing installed from it" { -not (Test-Path (Join-Path $bad "inst")) }

$foreign = Join-Path $env:TEMP ("tread-foreign-" + [guid]::NewGuid())
New-Item -ItemType Directory $foreign | Out-Null
Set-Content (Join-Path $foreign "keep.txt") "data"
Check "foreign directory refused" { -not (Run-Installer -Archive $sdk -Prefix $foreign -NoModifyPath) }
Check "foreign file kept" { (Get-Content (Join-Path $foreign "keep.txt")) -eq "data" }

$rtPrefix = Join-Path $env:TEMP ("tread-rt-" + [guid]::NewGuid())
Check "runtime install" { Run-Installer -Archive $rt -Prefix $rtPrefix -NoModifyPath }
Check "runtime version" { (& "$rtPrefix\bin\tread.exe" --version) -eq "tread $v1 (runtime)" }

Set-Content (Join-Path $prefix "notes.txt") "mine"
Check "upgrade to $v2" { Run-Installer -Archive $sdk2 }
Use-NewEnvironment
Check "upgraded version" { (tread --version) -eq "tread $v2" }
Check "user file kept on upgrade" { (Get-Content (Join-Path $prefix "notes.txt")) -eq "mine" }

Check "uninstall" { Run-Installer -Uninstall }
Check "files removed" { -not (Test-Path "$bin\tread.exe") }
Check "user file kept" { Test-Path (Join-Path $prefix "notes.txt") }
Check "PATH entry removed" { (Count-PathEntry $bin) -eq 0 }
Check "TREAD_HOME removed" { -not [Environment]::GetEnvironmentVariable("TREAD_HOME", "User") }

Finish "install.ps1"
