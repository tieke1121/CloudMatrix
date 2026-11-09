param(
    [string]$ReleaseDir = (Join-Path $PSScriptRoot '..\build\release')
)
$ErrorActionPreference = 'Stop'

$root = Get-Item -LiteralPath $ReleaseDir -Force
if (-not $root.PSIsContainer -or ($root.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
    throw '发行目录必须是普通目录'
}
$expected = @(
    'install.ps1', 'uninstall.ps1', 'verify-release.ps1',
    'bin/CloudMatrix.exe', 'bin/cloudmatrix-host.exe',
    'docs/runbook.md', 'licenses/THIRD-PARTY-LICENSES.md'
)
$items = @(Get-ChildItem -LiteralPath $root.FullName -Force -Recurse)
foreach ($item in $items) {
    if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) {
        throw "发行目录包含重解析点: $($item.Name)"
    }
    $relative = $item.FullName.Substring($root.FullName.Length + 1).Replace('\', '/')
    if ($item.PSIsContainer) {
        if ($relative -notin @('bin', 'docs', 'licenses')) { throw "发行目录含额外目录: $relative" }
    } elseif ($relative -notin ($expected + 'manifest.json')) {
        throw "发行目录含未授权文件: $relative"
    }
}

$manifest = Get-Content -Raw -LiteralPath (Join-Path $root.FullName 'manifest.json') | ConvertFrom-Json
if ($manifest.product -ne 'CloudMatrix') { throw '发行清单产品不匹配' }
$seen = @{}
foreach ($entry in $manifest.files) {
    $relative = [string]$entry.path
    if ($relative -notin $expected -or $seen.ContainsKey($relative)) {
        throw "发行清单含未授权或重复路径: $relative"
    }
    $seen[$relative] = $true
    $file = Get-Item -LiteralPath (Join-Path $root.FullName $relative)
    if ($file.PSIsContainer -or $file.Length -ne $entry.size_bytes) {
        throw "发行文件大小与清单不匹配: $relative"
    }
    if ((Get-FileHash -Algorithm SHA256 -LiteralPath $file.FullName).Hash -ne $entry.sha256) {
        throw "发行文件 SHA256 与清单不匹配: $relative"
    }
}
foreach ($required in $expected) {
    if (-not $seen.ContainsKey($required)) { throw "发行清单缺少文件: $required" }
}
Write-Host "发布包校验通过：$($seen.Count) 个文件，无额外文件。" -ForegroundColor Green
