# CloudMatrix 宿主容量基线与稳定性测试脚本
# 依据 docs/03-validation-plan.md 规范设计

param(
    [string]$InstanceId = "",
    [int]$DurationSeconds = 60,
    [string]$OutputDir = "artifacts\benchmarks"
)

$ErrorActionPreference = "Stop"

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "[BENCHMARK] 启动宿主资源采样与基准测试" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
}

$sampleFile = Join-Path $OutputDir "metrics_$(Get-Date -Format 'yyyyMMdd_HHmmss').csv"
"Timestamp,HostCPU_Percent,TotalRAM_GB,FreeRAM_GB,CommittedRAM_GB" | Out-File -FilePath $sampleFile -Encoding utf8

$stopTime = (Get-Date).AddSeconds($DurationSeconds)
$count = 0

while ((Get-Date) -lt $stopTime) {
    $os = Get-CimInstance Win32_OperatingSystem
    $cs = Get-CimInstance Win32_ComputerSystem
    $cpu = (Get-CimInstance Win32_Processor).LoadPercentage

    $totalRAM = [math]::Round($cs.TotalPhysicalMemory / 1GB, 2)
    $freeRAM = [math]::Round($os.FreePhysicalMemory / 1MB, 2)
    $committedRAM = [math]::Round(($os.TotalVirtualMemorySize - $os.FreeVirtualMemory) / 1MB, 2)
    $timeStr = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")

    "$timeStr,$cpu,$totalRAM,$freeRAM,$committedRAM" | Out-File -FilePath $sampleFile -Append -Encoding utf8
    $count++
    Start-Sleep -Seconds 1
}

Write-Host "[PASS] 采样完成，共采集 $count 条数据，文件: $sampleFile" -ForegroundColor Green
exit 0
