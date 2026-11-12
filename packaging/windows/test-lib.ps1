# Shared helpers for the Windows installer tests.
$script:Failures = 0

function Check([string]$name, [scriptblock]$test) {
    try {
        $ok = & $test
        if ($ok -is [array]) { $ok = $ok[-1] }
    } catch {
        Write-Host "      $_"
        $ok = $false
    }
    if ($ok) { Write-Host "ok    $name" } else { Write-Host "FAIL  $name"; $script:Failures++ }
}

function Test-SamePath([string]$a, [string]$b) {
    if (-not $a -or -not $b) { return $false }
    return $a.TrimEnd('\') -ieq $b.TrimEnd('\')
}

function Count-PathEntry([string]$dir) {
    $p = [Environment]::GetEnvironmentVariable("Path", "User")
    if (-not $p) { return 0 }
    return @($p -split ';' | Where-Object { Test-SamePath $_ $dir }).Count
}

# Use-NewEnvironment loads PATH and TREAD_HOME as a new terminal would see them.
function Use-NewEnvironment {
    $env:Path = [Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [Environment]::GetEnvironmentVariable("Path", "User")
    $env:TREAD_HOME = [Environment]::GetEnvironmentVariable("TREAD_HOME", "User")
}

function Finish([string]$what) {
    if ($script:Failures -ne 0) { Write-Host "$what tests: $($script:Failures) check(s) failed"; exit 1 }
    Write-Host "OK ($what)"
}
