$ErrorActionPreference = "Stop"
Set-Location -LiteralPath (Split-Path -Parent $MyInvocation.MyCommand.Path)

Write-Host "=== EasyLink Gateway Build ==="

Write-Host "[1/4] Preparing template from core..."
$templateDir = "gateway\template"
$coreDir = "core"

if (-not (Test-Path -LiteralPath $coreDir)) {
    Write-Error "core/ directory not found. Run from D:\Project\Easylink root."
    exit 1
}

Get-ChildItem -LiteralPath $coreDir -File | ForEach-Object {
    $dest = Join-Path $templateDir $_.Name
    Copy-Item -LiteralPath $_.FullName -Destination $dest -Force
}
Get-ChildItem -LiteralPath $coreDir -Directory | ForEach-Object {
    $dest = Join-Path $templateDir $_.Name
    if (Test-Path -LiteralPath $dest) {
        Remove-Item -LiteralPath $dest -Recurse -Force
    }
    Copy-Item -LiteralPath $_.FullName -Destination $dest -Recurse -Force
}

$templateDeviceIni = Join-Path $templateDir "Device.ini"
if (Test-Path -LiteralPath $templateDeviceIni) {
    Remove-Item -LiteralPath $templateDeviceIni -Force
}
Set-Content -LiteralPath $templateDeviceIni -Value "" -Encoding UTF8

$templateSetDef = Join-Path $templateDir "SetDef.fin"
if (Test-Path -LiteralPath $templateSetDef) {
    Remove-Item -LiteralPath $templateSetDef -Force
}

$templateLdb = Join-Path $templateDir "db_temp.ldb"
if (Test-Path -LiteralPath $templateLdb) {
    Remove-Item -LiteralPath $templateLdb -Force
}

$templateLog = Join-Path $templateDir "Log"
if (Test-Path -LiteralPath $templateLog) {
    Remove-Item -LiteralPath $templateLog -Recurse -Force -ErrorAction SilentlyContinue
}
New-Item -ItemType Directory -Path $templateLog -Force | Out-Null

Write-Host "[2/4] Running go mod tidy..."
Set-Location -LiteralPath "gateway"
go mod tidy
if ($LASTEXITCODE -ne 0) { throw "go mod tidy failed" }

Write-Host "[3/4] Building easylink-gate.exe..."
$env:CGO_ENABLED = "0"
go build -ldflags="-s -w" -o easylink-gate.exe .
if ($LASTEXITCODE -ne 0) { throw "go build failed" }

Write-Host "[4/4] Copying easylink-gate.exe to project root..."
Copy-Item -LiteralPath "easylink-gate.exe" -Destination "..\easylink-gate.exe" -Force
Set-Location -LiteralPath ".."

Write-Host "=== Build complete: easylink-gate.exe ==="
