[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Stage,
    [string]$OutputDir = 'dist'
)

$ErrorActionPreference = 'Stop'
$stagePath = (Resolve-Path -LiteralPath $Stage).ProviderPath
foreach ($relative in @('kapila-burner.exe', 'kapila-burner-web.exe',
    'gdk-pixbuf-query-loaders.exe', 'WINDOWS-RUNTIME-PACKAGES.txt',
    'tools\bin\ffmpeg.exe', 'tools\bin\ffprobe.exe', 'tools\bin\dvda-author.exe',
    'tools\bin\mkisofs.exe', 'tools\bin\vcdimager.exe')) {
    if (-not (Test-Path -LiteralPath (Join-Path $stagePath $relative) -PathType Leaf)) {
        throw "Incomplete Windows stage: missing $relative. Run bundle-windows.sh first."
    }
}

$compiler = Get-Command 'ISCC.exe' -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
$iscc = if ($compiler) { $compiler.Source } else { $null }
if (-not $iscc) {
    foreach ($base in @(${env:ProgramFiles(x86)}, $env:ProgramFiles)) {
        if (-not $base) { continue }
        foreach ($version in @('7', '6')) {
            $candidate = Join-Path $base "Inno Setup $version\ISCC.exe"
            if (Test-Path -LiteralPath $candidate -PathType Leaf) {
                $iscc = $candidate
                break
            }
        }
        if ($iscc) { break }
    }
}
if (-not $iscc) {
    throw 'ISCC.exe not found. Install Inno Setup in CI with: choco install innosetup --yes --no-progress'
}

$null = New-Item -ItemType Directory -Force -Path $OutputDir
$outputPath = (Resolve-Path -LiteralPath $OutputDir).ProviderPath
$installer = Join-Path $outputPath 'kapila-burner-windows-amd64-setup.exe'
# An unsuccessful compile must never be mistaken for an older successful build.
if (Test-Path -LiteralPath $installer) { Remove-Item -LiteralPath $installer }
& $iscc "/DSourceDir=$stagePath" "/DOutputDir=$outputPath" (Join-Path $PSScriptRoot 'windows-installer.iss')
if ($LASTEXITCODE -ne 0) { throw "Inno Setup compilation failed with exit code $LASTEXITCODE." }
if (-not (Test-Path -LiteralPath $installer -PathType Leaf)) {
    throw "Inno Setup did not produce the expected installer: $installer"
}
Write-Output $installer
