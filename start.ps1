# ============================================
#  NightCode - Start All Services
#  Frontend (5173) + Backend (3001) + ds2api (5001)
# ============================================

$root = "C:\Users\Mubasher Developer\Desktop"
$ds2apiDir = "$root\ds2api"
$backendDir = "$root\NightCode\Backend"
$frontendDir = "$root\NightCode\Frontend"

Write-Host ""
Write-Host "  ========================================" -ForegroundColor Cyan
Write-Host "   NightCode - Starting all services..." -ForegroundColor Cyan
Write-Host "  ========================================" -ForegroundColor Cyan
Write-Host ""

# ds2api
Write-Host "[1/3] Starting ds2api on :5001 ..." -ForegroundColor Yellow
Start-Process -FilePath "cmd.exe" -ArgumentList "/c", "cd /d `"$ds2apiDir`" && ds2api.exe" -WindowStyle Minimized

# Backend
Write-Host "[2/3] Starting backend on :3001 ..." -ForegroundColor Yellow
Start-Process -FilePath "cmd.exe" -ArgumentList "/c", "cd /d `"$backendDir`" && server.exe" -WindowStyle Minimized

# Frontend
Write-Host "[3/3] Starting frontend on :5173 ..." -ForegroundColor Yellow
Start-Process -FilePath "cmd.exe" -ArgumentList "/c", "cd /d `"$frontendDir`" && npm run dev" -WindowStyle Normal

Write-Host ""
Write-Host "  ========================================" -ForegroundColor Green
Write-Host "   All services started!" -ForegroundColor Green
Write-Host "   Frontend:  http://localhost:5173" -ForegroundColor White
Write-Host "   Backend:   http://localhost:3001" -ForegroundColor White
Write-Host "   ds2api:    http://localhost:5001" -ForegroundColor White
Write-Host "  ========================================" -ForegroundColor Green
Write-Host ""
Write-Host "  Press Enter to stop all services..." -ForegroundColor DarkGray
Read-Host

# Stop all
Get-Process -Name "ds2api" -ErrorAction SilentlyContinue | Stop-Process -Force
Get-Process -Name "server" -ErrorAction SilentlyContinue | Stop-Process -Force
Get-Process -Name "node" -ErrorAction SilentlyContinue | Where-Object { $_.CommandLine -match "vite" } | Stop-Process -Force
Write-Host "All services stopped." -ForegroundColor Red
