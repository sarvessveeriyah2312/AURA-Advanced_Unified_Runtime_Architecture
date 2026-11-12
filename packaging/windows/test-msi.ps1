<#
.SYNOPSIS
Tests the Tread SDK .msi end to end (ADR-0011 §7): per-user install, user
PATH and TREAD_HOME, tread doctor, upgrade, and uninstall. CI only.

  packaging\windows\test-msi.ps1 -Dir installers -UpgradeDir installers-upgrade
#>
param(
    [Parameter(Mandatory = $true)][string]$Dir,
    [Parameter(Mandatory = $true)][string]$UpgradeDir
)
$ErrorActionPreference = "Stop"
. (Join-Path $PSScriptRoot "test-lib.ps1")

$msi = (Get-ChildItem "$Dir\tread-sdk-*-windows-x64.msi" | Select-Object -First 1).FullName
$up = (Get-ChildItem "$UpgradeDir\tread-sdk-*-windows-x64.msi" | Select-Object -First 1).FullName
$v1 = ([IO.Path]::GetFileName($msi) -replace '^tread-sdk-', '' -replace '-windows-x64\.msi$', '')
$v2 = ([IO.Path]::GetFileName($up) -replace '^tread-sdk-', '' -replace '-windows-x64\.msi$', '')
$home_ = Join-Path $env:LOCALAPPDATA "Programs\Tread"
$bin = Join-Path $home_ "bin"

function Invoke-Msi([string]$verb, [string]$path) {
    $log = Join-Path $env:TEMP ("msi-" + [guid]::NewGuid() + ".log")
    $p = Start-Process msiexec.exe -ArgumentList "$verb `"$path`" /qn /norestart /l*v `"$log`"" -Wait -PassThru
    if ($p.ExitCode -ne 0) { Get-Content $log -Tail 40 | Write-Host; throw "msiexec $verb exited $($p.ExitCode)" }
    return $true
}

Check "install $v1" { Invoke-Msi "/i" $msi }
Check "binary installed" { Test-Path "$bin\tread.exe" }
Check "user PATH has bin once" { (Count-PathEntry $bin) -eq 1 }
Check "TREAD_HOME set" { Test-SamePath ([Environment]::GetEnvironmentVariable("TREAD_HOME", "User")) $home_ }
Use-NewEnvironment
Check "version" { (tread --version) -eq "tread $v1" }
Check "doctor healthy" { $o = (tread doctor | Out-String); Write-Host $o; $o -match "Installation is healthy" }
Check "doctor PATH ok" { (tread doctor | Out-String) -match "\[OK\]   PATH: tread resolves to this executable" }
Check "doctor TREAD_HOME ok" { (tread doctor | Out-String) -match "\[OK\]   TREAD_HOME: " }
$work = Join-Path $env:TEMP ("tread-msi-" + [guid]::NewGuid())
New-Item -ItemType Directory $work | Out-Null
Check "new, build, run" {
    Push-Location $work
    try { tread new Demo | Out-Null; tread build Demo\src\main.tr; (tread run Demo\src\main.tbc) -eq "Hello from Demo!" } finally { Pop-Location }
}

Check "upgrade to $v2" { Invoke-Msi "/i" $up }
Use-NewEnvironment
Check "upgraded version" { (tread --version) -eq "tread $v2" }
Check "PATH still has bin once" { (Count-PathEntry $bin) -eq 1 }

Check "uninstall" { Invoke-Msi "/x" $up }
Check "files removed" { -not (Test-Path "$bin\tread.exe") }
Check "PATH entry removed" { (Count-PathEntry $bin) -eq 0 }
Check "TREAD_HOME removed" { -not [Environment]::GetEnvironmentVariable("TREAD_HOME", "User") }
Check "project kept" { Test-Path "$work\Demo\src\main.tr" }

Finish ".msi"
