param([switch]$PurgeAllUserData)
$ErrorActionPreference = 'Stop'
if ($PurgeAllUserData) { throw '本脚本不提供用户数据清除操作' }

$root = Get-Item -LiteralPath $PSScriptRoot -Force
for ($parent = $root.Parent; $parent; $parent = $parent.Parent) {
    if ($parent.Attributes -band [IO.FileAttributes]::ReparsePoint) {
        throw "拒绝从重解析路径卸载: $($parent.FullName)"
    }
}
$binPath = Join-Path $root.FullName 'bin'
$bin = Get-Item -LiteralPath $binPath -Force -ErrorAction SilentlyContinue
$manifestPath = Join-Path $root.FullName 'manifest.json'
$manifestFile = Get-Item -LiteralPath $manifestPath -Force
foreach ($entry in @($root, $bin, $manifestFile)) {
    if ($entry -and ($entry.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        throw "拒绝从重解析路径卸载: $($entry.FullName)"
    }
}
$manifest = Get-Content -Raw -LiteralPath $manifestPath | ConvertFrom-Json
if ($manifest.product -ne 'CloudMatrix') { throw '卸载目录缺少 CloudMatrix 安装清单' }

$files = @()
foreach ($name in @('CloudMatrix.exe', 'cloudmatrix-host.exe')) {
    $file = Get-Item -LiteralPath (Join-Path $binPath $name) -Force -ErrorAction SilentlyContinue
    if (-not $file) { continue }
    if ($file.PSIsContainer -or ($file.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        throw "拒绝删除非常规文件: $($file.FullName)"
    }
    $owned = @($manifest.files | Where-Object path -eq "bin/$name")
    if ($owned.Count -ne 1 -or (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash -ne $owned[0].sha256) {
        throw "拒绝移除无法确认归属或已改动的文件: $($file.FullName)"
    }
    $files += $file
}
foreach ($file in $files) {
    Remove-Item -LiteralPath $file.FullName -Force
}
Write-Host '程序可执行文件已移除，用户数据已保留。' -ForegroundColor Green
