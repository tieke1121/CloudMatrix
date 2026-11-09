# CloudMatrix 全局检查入口脚本
# 依据 docs/02-agent-implementation-plan.md 规范

$ErrorActionPreference = "Stop"

# 自动补充 Go 和 Wails 路径
$goPath = "D:\soft\go\bin"
$goUserBin = "$env:USERPROFILE\go\bin"
if ($env:Path -notmatch [regex]::Escape($goPath)) {
    $env:Path = "$goPath;$env:Path"
}
if ($env:Path -notmatch [regex]::Escape($goUserBin)) {
    $env:Path = "$goUserBin;$env:Path"
}

function Run-Step {
    param(
        [string]$Name,
        [scriptblock]$Script
    )
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host "[CHECK] 正在执行: $Name" -ForegroundColor Cyan
    Write-Host "==========================================" -ForegroundColor Cyan
    & $Script
    $code = $LASTEXITCODE
    if ($code -ne 0) {
        Write-Host "------------------------------------------" -ForegroundColor Red
        Write-Host "[FAIL] $Name 失败，退出码: $code" -ForegroundColor Red
        Write-Host "------------------------------------------" -ForegroundColor Red
        exit $code
    }
    Write-Host "[PASS] $Name 完成" -ForegroundColor Green
}

# 2. 前端类型检查
Run-Step "前端类型检查 (typecheck)" {
    npm --prefix frontend run typecheck
}

# 3. 前端单元测试
Run-Step "前端单元测试 (test:unit)" {
    npm --prefix frontend run test:unit -- --pool=threads --poolOptions.threads.singleThread
}

# 4. 前端构建
Run-Step "前端打包构建 (build)" {
    npm --prefix frontend run build
}

Run-Step "Go 单元测试" {
    go test ./... -count=1
}

# 5. 桌面端构建
Run-Step "桌面端应用构建" {
    $wailsCmd = Get-Command wails -ErrorAction SilentlyContinue
    if ($wailsCmd) {
        wails build -platform windows/amd64
    } else {
        Write-Host "提示: Wails CLI 未在 PATH，使用原生 Go 构建桌面可执行文件..." -ForegroundColor Yellow
        if (-not (Test-Path "build\bin")) {
            New-Item -ItemType Directory -Path "build\bin" -Force | Out-Null
        }
        go build -tags production -ldflags '-H windowsgui' -o build\bin\CloudMatrix.exe .
    }
}

Write-Host "==========================================" -ForegroundColor Green
Write-Host "所有验证步骤全部通过！" -ForegroundColor Green
Write-Host "==========================================" -ForegroundColor Green
exit 0
