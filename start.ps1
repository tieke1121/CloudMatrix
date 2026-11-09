param([string]$ReleaseDir)
$ErrorActionPreference = 'Stop'

if (-not $ReleaseDir) {
    $latest = Get-ChildItem -LiteralPath (Join-Path $PSScriptRoot 'build\releases') -Directory -ErrorAction SilentlyContinue |
        Where-Object { Test-Path -LiteralPath (Join-Path $_.FullName 'manifest.json') -PathType Leaf } |
        Sort-Object LastWriteTimeUtc -Descending | Select-Object -First 1
    if ($latest) { $ReleaseDir = $latest.FullName }
}
if ($ReleaseDir) {
    & (Join-Path $PSScriptRoot 'scripts\verify-release.ps1') -ReleaseDir $ReleaseDir
    $exe = (Resolve-Path -LiteralPath (Join-Path $ReleaseDir 'bin\CloudMatrix.exe')).Path
    Start-Process -FilePath $exe
    exit 0
}

Write-Host '[INFO] 未找到发行包，构建前端后从源码启动。'
Push-Location $PSScriptRoot
try {
    npm --prefix frontend run build
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    go run -tags production .
    exit $LASTEXITCODE
} finally {
    Pop-Location
}
