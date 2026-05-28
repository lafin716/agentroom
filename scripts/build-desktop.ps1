$ErrorActionPreference = "Stop"

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$BinDir = Join-Path $RepoRoot "apps\desktop\src-tauri\binaries"
$DesktopDir = Join-Path $RepoRoot "apps\desktop"

if (-not (Test-Path $BinDir)) {
    New-Item -ItemType Directory -Path $BinDir | Out-Null
}

# Tauri sidecars require a target-triple suffix.
$TripleRaw = & rustc -vV | Select-String -Pattern "^host:" | ForEach-Object { ($_ -split ":")[1].Trim() }
if (-not $TripleRaw) {
    Write-Error "could not determine rustc host triple"
}
$Triple = $TripleRaw

Write-Host ">> Building ar CLI for triple $Triple"
$OutPath = Join-Path $BinDir "ar-$Triple.exe"
Push-Location (Join-Path $RepoRoot "apps\cli")
try {
    go build -o $OutPath .
} finally {
    Pop-Location
}

Write-Host ">> Sidecar at $OutPath"

if ($args -contains "--dev") {
    Push-Location $DesktopDir
    try { pnpm tauri dev } finally { Pop-Location }
} elseif ($args -contains "--build") {
    Push-Location $DesktopDir
    try { pnpm tauri build } finally { Pop-Location }
} else {
    Write-Host ""
    Write-Host "Next steps:"
    Write-Host "  cd apps/desktop"
    Write-Host "  pnpm install     # first time only"
    Write-Host "  pnpm tauri dev   # run app"
    Write-Host "  pnpm tauri build # produce installer"
}
