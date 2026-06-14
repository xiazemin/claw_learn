# GoClaw Channel Bridge - One-click Startup Script
# Usage:
#   .\start.ps1                          # HTTP server mode
#   .\start.ps1 -Interactive             # Interactive CLI mode
#   .\start.ps1 -Build -Interactive      # Rebuild + interactive
#   .\start.ps1 -Port 28800              # Custom port

param(
    [switch]$Build,
    [switch]$Interactive,
    [string]$Config = "config.json",
    [int]$Port = 19900
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$Binary = Join-Path $ScriptDir "channel-bridge.exe"

Write-Host ""
if ($Interactive) {
    Write-Host "  GoClaw Interactive CLI" -ForegroundColor Cyan
} else {
    Write-Host "  GoClaw Channel Bridge Starter" -ForegroundColor Cyan
}
Write-Host ""

# Step 1: Build if requested or binary doesn't exist
if ($Build -or !(Test-Path $Binary)) {
    Write-Host "[1/4] Building channel bridge..." -ForegroundColor Yellow
    Push-Location $ScriptDir
    try {
        go build -o "channel-bridge.exe" .
        if ($LASTEXITCODE -ne 0) {
            Write-Host "  X Build failed!" -ForegroundColor Red
            exit 1
        }
        Write-Host "  OK Build succeeded" -ForegroundColor Green
    } finally {
        Pop-Location
    }
} else {
    Write-Host "[1/4] Binary exists, skipping build (use -Build to force)" -ForegroundColor Gray
}

# Step 2: Kill existing bridge processes on the target port
if (-not $Interactive) {
    Write-Host "[2/4] Checking port $Port..." -ForegroundColor Yellow
    $existing = Get-NetTCPConnection -LocalPort $Port -ErrorAction SilentlyContinue |
        Select-Object -ExpandProperty OwningProcess -Unique
    if ($existing) {
        foreach ($procId in $existing) {
            $proc = Get-Process -Id $procId -ErrorAction SilentlyContinue
            if ($proc -and $proc.Name -eq "channel-bridge") {
                Write-Host "  Stopping existing bridge (PID $procId)..." -ForegroundColor Yellow
                Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue
                Start-Sleep -Seconds 2
            }
        }
    }
    Write-Host "  OK Port $Port is available" -ForegroundColor Green
} else {
    Write-Host "[2/4] Skipping port check (interactive mode)" -ForegroundColor Gray
}

# Step 3: Check goclaw is running
Write-Host "[3/4] Checking goclaw gateway..." -ForegroundColor Yellow
$GoclawRunning = $false
try {
    $response = Invoke-WebRequest -Uri "http://localhost:18790/health" -TimeoutSec 3 -ErrorAction SilentlyContinue
    if ($response.StatusCode -eq 200) {
        $GoclawRunning = $true
        Write-Host "  OK GoClaw gateway is running" -ForegroundColor Green
    }
} catch {
    # Ignore
}
if (-not $GoclawRunning) {
    Write-Host "  WARNING: GoClaw gateway not detected at http://localhost:18790" -ForegroundColor Yellow
    Write-Host "  Starting anyway (will retry connection)..." -ForegroundColor Yellow
}

# Step 4: Start
if ($Interactive) {
    Write-Host "[4/4] Starting interactive CLI..." -ForegroundColor Yellow
    Write-Host ""
    Push-Location $ScriptDir
    try {
        & $Binary -config $Config -i
    } finally {
        Pop-Location
    }
} else {
    Write-Host "[4/4] Starting channel bridge on port $Port..." -ForegroundColor Yellow
    Write-Host ""
    Write-Host "  Bridge API:  http://localhost:$Port" -ForegroundColor White
    Write-Host "  GoClaw:      http://localhost:18790" -ForegroundColor White
    Write-Host ""
    $env:GOCLAW_LISTEN_ADDR = ":$Port"
    Push-Location $ScriptDir
    try {
        & $Binary -config $Config
    } finally {
        Pop-Location
    }
}
