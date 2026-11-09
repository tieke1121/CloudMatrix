# ADB preflight for an explicitly selected isolated test instance.
param(
    [Parameter(Mandatory = $true)]
    [ValidateNotNullOrEmpty()]
    [string]$InstanceId,

    [Parameter(Mandatory = $true)]
    [ValidatePattern('^\S+$')]
    [string]$Serial,

    [string]$AdbPath = 'adb',
    [switch]$AdbPreflightOnly
)
$ErrorActionPreference = 'Stop'

function Write-Result([string]$Status, [string]$Step, [int]$Code, [string]$Detail) {
    [ordered]@{
        instance_id = $InstanceId
        case = $Step
        status = $Status
        exit_code = $Code
        detail = $Detail
    } | ConvertTo-Json -Compress
}

$adb = Get-Command -Name $AdbPath -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
if (-not $adb) {
    Write-Result 'BLOCKED' 'adb_preflight' 2 '找不到 ADB'
    exit 2
}

try {
    $devices = @(& $adb.Source devices -l)
    if ($LASTEXITCODE -ne 0) { throw "adb devices 退出码 $LASTEXITCODE" }
    $device = $devices | ForEach-Object { ([string]$_).Trim() -split '\s+', 3 -join ' ' } |
        Where-Object { $_ -cmatch ('^' + [regex]::Escape($Serial) + ' device(?: |$)') } | Select-Object -First 1
    if (-not $device) { throw '指定 serial 不在线或未授权' }

    $boot = @(& $adb.Source -s $Serial shell getprop sys.boot_completed)
    if ($LASTEXITCODE -ne 0 -or ($boot -join '').Trim() -ne '1') { throw 'Android 尚未启动完成' }

    $packages = @(& $adb.Source -s $Serial shell pm path android)
    if ($LASTEXITCODE -ne 0 -or -not ($packages -match '^package:')) { throw '包管理器未就绪' }
} catch {
    Write-Result 'FAIL' 'adb_preflight' 1 $_.Exception.Message
    exit 1
}

Write-Result 'PASS' 'adb_preflight' 0 '指定 ADB serial 在线，启动完成，包管理器可用'
if (-not $AdbPreflightOnly) {
    Write-Result 'NOT_RUN' 'emulator_lifecycle' 2 '该脚本仅运行只读 ADB 预检，未执行真实模拟器生命周期验收'
    exit 2
}
exit 0
