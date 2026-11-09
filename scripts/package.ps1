param([string]$ReleaseDir)
$ErrorActionPreference = 'Stop'

$root = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
if ([string]::IsNullOrWhiteSpace($ReleaseDir)) {
    $stamp = [DateTime]::UtcNow.ToString('yyyyMMdd-HHmmss')
    $suffix = [Guid]::NewGuid().ToString('N').Substring(0, 8)
    $ReleaseDir = Join-Path $root "build\releases\$stamp-$suffix"
}
$ReleaseDir = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($ReleaseDir)
if (Test-Path -LiteralPath $ReleaseDir) { throw "发行目录已存在，请选择新目录: $ReleaseDir" }

Push-Location (Join-Path $root 'frontend')
try {
    npm run build
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} finally {
    Pop-Location
}

New-Item -ItemType Directory -Path $ReleaseDir | Out-Null
$bin = New-Item -ItemType Directory -Path (Join-Path $ReleaseDir 'bin')
Push-Location $root
try {
    go build -tags production -ldflags '-s -w -H windowsgui' -o (Join-Path $bin.FullName 'CloudMatrix.exe') .
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    go build -ldflags '-s -w' -o (Join-Path $bin.FullName 'cloudmatrix-host.exe') ./cmd/cloudmatrix-host
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} finally {
    Pop-Location
}

foreach ($folder in @('docs', 'licenses')) {
    New-Item -ItemType Directory -Path (Join-Path $ReleaseDir $folder) | Out-Null
}
Copy-Item -LiteralPath (Join-Path $root 'docs\release\runbook.md') -Destination (Join-Path $ReleaseDir 'docs')
Copy-Item -LiteralPath (Join-Path $root 'licenses\THIRD-PARTY-LICENSES.md') -Destination (Join-Path $ReleaseDir 'licenses')
foreach ($script in @('install.ps1', 'uninstall.ps1')) {
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot "release\$script") -Destination $ReleaseDir
}
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'verify-release.ps1') -Destination $ReleaseDir

$files = @(Get-ChildItem -LiteralPath $ReleaseDir -Recurse -Force -File | ForEach-Object {
    @{
        path = $_.FullName.Substring($ReleaseDir.Length + 1).Replace('\', '/')
        size_bytes = $_.Length
        sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $_.FullName).Hash.ToLowerInvariant()
    }
})
@{
    product = 'CloudMatrix'
    version = (Get-Content -Raw -LiteralPath (Join-Path $root 'wails.json') | ConvertFrom-Json).info.productVersion
    created_at_utc = [DateTime]::UtcNow.ToString('o')
    platform = 'windows-amd64'
    files = $files
} | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $ReleaseDir 'manifest.json') -Encoding utf8

& (Join-Path $PSScriptRoot 'verify-release.ps1') -ReleaseDir $ReleaseDir
Write-Host "发布包生成成功: $ReleaseDir" -ForegroundColor Green
