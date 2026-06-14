# GoClaw Channel Bridge - Test Script
# Run after starting the bridge with start.ps1

param(
    [string]$BaseUrl = "http://localhost:18800"
)

$ErrorActionPreference = "Stop"

Write-Host ""
Write-Host "  GoClaw Channel Bridge - Test Suite" -ForegroundColor Cyan
Write-Host "  Target: $BaseUrl" -ForegroundColor Gray
Write-Host ""

# Test 1: Health check
Write-Host "[Test 1] Health Check" -ForegroundColor Yellow
try {
    $health = Invoke-RestMethod -Uri "$BaseUrl/health" -Method GET
    Write-Host "  Bridge: $($health.bridge)" -ForegroundColor $(if ($health.bridge -eq "ok") { "Green" } else { "Red" })
    Write-Host "  Connected: $($health.connected)" -ForegroundColor $(if ($health.connected) { "Green" } else { "Red" })
} catch {
    Write-Host "  ✗ Failed: $_" -ForegroundColor Red
    return
}

# Test 2: Status
Write-Host "`n[Test 2] Status" -ForegroundColor Yellow
try {
    $status = Invoke-RestMethod -Uri "$BaseUrl/status" -Method GET
    Write-Host "  Service: $($status.service) v$($status.version)" -ForegroundColor Green
    Write-Host "  Goclaw URL: $($status.goclaw_url)" -ForegroundColor Gray
    Write-Host "  Agent Key: $($status.agent_key)" -ForegroundColor Gray
} catch {
    Write-Host "  ✗ Failed: $_" -ForegroundColor Red
}

# Test 3: List Models
Write-Host "`n[Test 3] List Models" -ForegroundColor Yellow
try {
    $models = Invoke-RestMethod -Uri "$BaseUrl/models" -Method GET
    if ($models.ok) {
        Write-Host "  ✓ Models retrieved" -ForegroundColor Green
        # Print first few
        if ($models.payload -is [array]) {
            $models.payload | Select-Object -First 3 | ForEach-Object {
                Write-Host "    - $($_.name)" -ForegroundColor Gray
            }
        }
    } else {
        Write-Host "  ⚠ Response: $($models | ConvertTo-Json -Compress)" -ForegroundColor Yellow
    }
} catch {
    Write-Host "  ✗ Failed: $_" -ForegroundColor Red
}

# Test 4: List Skills
Write-Host "`n[Test 4] List Skills" -ForegroundColor Yellow
try {
    $skills = Invoke-RestMethod -Uri "$BaseUrl/skills" -Method GET
    if ($skills.ok) {
        Write-Host "  ✓ Skills retrieved" -ForegroundColor Green
    } else {
        Write-Host "  ⚠ Response: $($skills | ConvertTo-Json -Compress)" -ForegroundColor Yellow
    }
} catch {
    Write-Host "  ✗ Failed: $_" -ForegroundColor Red
}

# Test 5: Chat (sync)
Write-Host "`n[Test 5] Chat (Sync)" -ForegroundColor Yellow
$body = @{
    content = "Hello! Please respond with just 'Hello from channel bridge!' and nothing else."
} | ConvertTo-Json

try {
    Write-Host "  Sending message..." -ForegroundColor Gray
    $chat = Invoke-RestMethod -Uri "$BaseUrl/chat" -Method POST -Body $body -ContentType "application/json" -TimeoutSec 60
    if ($chat.ok) {
        Write-Host "  ✓ Response ($($chat.elapsed)):" -ForegroundColor Green
        Write-Host "    $($chat.content)" -ForegroundColor White
    } else {
        Write-Host "  ✗ Error: $($chat.error)" -ForegroundColor Red
    }
} catch {
    Write-Host "  ✗ Failed: $_" -ForegroundColor Red
}

# Test 6: Chat (streaming SSE)
Write-Host "`n[Test 6] Chat (Streaming SSE)" -ForegroundColor Yellow
$body = @{
    content = "Count from 1 to 5, one number per line."
} | ConvertTo-Json

try {
    Write-Host "  Sending streaming message..." -ForegroundColor Gray
    # Use curl for SSE since Invoke-RestMethod doesn't handle SSE well
    $tempFile = [System.IO.Path]::GetTempFileName()
    $curlPath = "C:\Users\wps\Downloads\curl\curl-8.20.0_3-win64-mingw\bin\curl.exe"
    
    if (Test-Path $curlPath) {
        & $curlPath -s -N -X POST "$BaseUrl/chat/stream" `
            -H "Content-Type: application/json" `
            -d $body `
            --max-time 120 2>$null | ForEach-Object {
            if ($_ -match "^data: (.+)$") {
                $json = $matches[1] | ConvertFrom-Json -ErrorAction SilentlyContinue
                if ($json.content) {
                    Write-Host "    $($json.content)" -ForegroundColor White -NoNewline
                }
                if ($json.done) {
                    Write-Host ""
                }
                if ($json.error) {
                    Write-Host "  ✗ Stream error: $($json.error)" -ForegroundColor Red
                }
            }
        }
        Write-Host "  ✓ Streaming completed" -ForegroundColor Green
    } else {
        Write-Host "  ⚠ curl not found, using Invoke-RestMethod (no streaming)" -ForegroundColor Yellow
        $chat = Invoke-RestMethod -Uri "$BaseUrl/chat" -Method POST -Body $body -ContentType "application/json" -TimeoutSec 60
        if ($chat.ok) {
            Write-Host "  ✓ Response: $($chat.content)" -ForegroundColor Green
        }
    }
} catch {
    Write-Host "  ✗ Failed: $_" -ForegroundColor Red
}

Write-Host "`n  All tests completed!" -ForegroundColor Cyan
Write-Host ""
