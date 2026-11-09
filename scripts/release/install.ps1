param([string]$InstallPath = 'C:\Program Files\CloudMatrix')
$ErrorActionPreference = 'Stop'

& (Join-Path $PSScriptRoot 'verify-release.ps1') -ReleaseDir $PSScriptRoot
$manifest = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot 'manifest.json') | ConvertFrom-Json
$InstallPath = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($InstallPath)
if ($InstallPath -eq $PSScriptRoot) { throw '请选择其他安装目录' }

$paths = @($manifest.files.path) + 'manifest.json'
foreach ($relative in $paths) {
    $target = Join-Path $InstallPath $relative
    $existing = Get-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
    if ($existing -and ($existing.PSIsContainer -or ($existing.Attributes -band [IO.FileAttributes]::ReparsePoint))) {
        throw "安装目标不是普通文件: $target"
    }
    for ($folder = Split-Path -Parent $target; $folder; $folder = [IO.Path]::GetDirectoryName($folder)) {
        $existing = Get-Item -LiteralPath $folder -Force -ErrorAction SilentlyContinue
        if ($existing -and (-not $existing.PSIsContainer -or ($existing.Attributes -band [IO.FileAttributes]::ReparsePoint))) {
            throw "安装路径不是普通目录: $folder"
        }
    }
}
$oldManifestPath = Join-Path $InstallPath 'manifest.json'
$owned = @{}
if (Test-Path -LiteralPath $oldManifestPath -PathType Leaf) {
    $old = Get-Content -Raw -LiteralPath $oldManifestPath | ConvertFrom-Json
    if ($old.product -ne 'CloudMatrix') { throw '已有清单不属于 CloudMatrix' }
    foreach ($entry in $old.files) {
        if ($entry.path -in $paths -and $entry.path -ne 'manifest.json') {
            if ($owned.ContainsKey($entry.path)) { throw '已有清单包含重复路径' }
            $owned[$entry.path] = $entry.sha256
        }
    }
    if (-not $owned.ContainsKey('bin/CloudMatrix.exe') -or -not $owned.ContainsKey('bin/cloudmatrix-host.exe')) {
        throw '已有清单无法确认安装归属'
    }
}
foreach ($relative in $manifest.files.path) {
    $target = Join-Path $InstallPath $relative
    if ((Test-Path -LiteralPath $target -PathType Leaf) -and
        (!$owned.ContainsKey($relative) -or (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash -ne $owned[$relative])) {
        throw "拒绝覆盖无法确认归属或已改动的文件: $target"
    }
}
foreach ($relative in $paths) {
    $target = Join-Path $InstallPath $relative
    New-Item -ItemType Directory -Path (Split-Path -Parent $target) -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot $relative) -Destination $target -Force
}
Write-Host "安装完成：$InstallPath\bin\CloudMatrix.exe" -ForegroundColor Green
