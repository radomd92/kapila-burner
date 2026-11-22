[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Installer,
    [Parameter(Mandatory = $true)]
    [string]$Python,
    [Parameter(Mandatory = $true)]
    [string]$SmokeScriptPath
)

$ErrorActionPreference = 'Stop'
if ($env:OS -ne 'Windows_NT') { throw 'Run this test in native Windows PowerShell.' }
foreach ($name in @('Installer', 'Python', 'SmokeScriptPath')) {
    $path = (Get-Variable -Name $name -ValueOnly)
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Missing ${name}: $path" }
    Set-Variable -Name $name -Value (Resolve-Path -LiteralPath $path).ProviderPath
}
if (-not $env:RUNNER_TEMP -or -not (Test-Path -LiteralPath $env:RUNNER_TEMP -PathType Container)) {
    throw 'RUNNER_TEMP must name an existing temporary directory.'
}
# This AppID also owns Start Menu entries: use an ephemeral CI user, never a real installation.
$registration = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\{EF78D7AB-1B46-4B02-9432-14B55AE9B407}_is1'
if (Test-Path -LiteralPath $registration) { throw 'kapila-maker is already installed for this user.' }

function Invoke-SetupProcess([string]$Executable, [string[]]$Arguments) {
    # WaitForExit, rather than Start-Process -Wait, bounds a hung installer/uninstaller.
    $process = Start-Process -FilePath $Executable -ArgumentList $Arguments -PassThru
    try {
        if (-not $process.WaitForExit(120000)) {
            $process.Kill()
            $process.WaitForExit()
            throw "Installer process timed out after 120 seconds: $Executable"
        }
        if ($process.ExitCode -ne 0) { throw "Installer process exited with code $($process.ExitCode): $Executable" }
    } finally {
        $process.Dispose()
    }
}

$installed = Join-Path (Resolve-Path -LiteralPath $env:RUNNER_TEMP).ProviderPath ("kapila installed " + [guid]::NewGuid().ToString('N'))
$null = New-Item -ItemType Directory -Path $installed
$nativeExe = Join-Path $installed 'kapila-maker.exe'
$uninstaller = Join-Path $installed 'unins000.exe'
try {
    Invoke-SetupProcess $Installer @('/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART', '/SP-', "/DIR=`"$installed`"")
    foreach ($relative in @('kapila-maker.exe', 'kapila-maker-web.exe', 'gdk-pixbuf-query-loaders.exe',
        'tools\bin\ffmpeg.exe', 'tools\bin\ffprobe.exe', 'tools\bin\dvda-author.exe',
        'tools\bin\mkisofs.exe', 'tools\bin\vcdimager.exe',
        'share\glib-2.0\schemas\gschemas.compiled', 'unins000.exe')) {
        if (-not (Test-Path -LiteralPath (Join-Path $installed $relative) -PathType Leaf)) {
            throw "Installed payload is missing $relative"
        }
    }
    # The authoring data directory must survive packaging even when it is empty.
    if (-not (Test-Path -LiteralPath (Join-Path $installed 'tools\share\dvda-author') -PathType Container)) {
        throw 'Installed payload is missing tools\share\dvda-author'
    }
    & $Python -B $SmokeScriptPath $installed
    if ($LASTEXITCODE -ne 0) { throw "Installed bundle smoke test failed with exit code $LASTEXITCODE." }
} finally {
    try {
        if (Test-Path -LiteralPath $uninstaller -PathType Leaf) {
            Invoke-SetupProcess $uninstaller @('/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART', '/SP-')
        }
        if (Test-Path -LiteralPath $nativeExe) { throw 'Uninstall did not remove kapila-maker.exe.' }
        if (Test-Path -LiteralPath $registration) { throw 'Uninstall did not remove the per-user Inno registration.' }
    } finally {
        # Only this invocation's newly created GUID directory is eligible for deletion.
        if (Test-Path -LiteralPath $installed) { Remove-Item -LiteralPath $installed -Recurse -Force }
    }
}
Write-Output 'Installed Windows payload, smoke test, and uninstall PASS; no physical burning.'
