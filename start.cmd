@echo off
REM ============================================
REM  NightCode - Start All Services
REM  Frontend (5173) + Backend (3001) + ds2api (5001)
REM ============================================

echo.
echo  ========================================
echo   NightCode - Starting all services...
echo  ========================================
echo.

REM --- ds2api (DeepSeek Web API) ---
echo [1/3] Starting ds2api on :5001 ...
start "ds2api" /min cmd /c "cd /d C:\Users\Mubasher Developer\Desktop\ds2api && ds2api.exe"

REM --- Backend (Go API server) ---
echo [2/3] Starting backend on :3001 ...
start "nightcode-backend" /min cmd /c "cd /d C:\Users\Mubasher Developer\Desktop\NightCode\Backend && server.exe"

REM --- Frontend (Vite dev server) ---
echo [3/3] Starting frontend on :5173 ...
start "nightcode-frontend" cmd /c "cd /d C:\Users\Mubasher Developer\Desktop\NightCode\Frontend && npm run dev"

echo.
echo  ========================================
echo   All services started!
echo   Frontend:  http://localhost:5173
echo   Backend:   http://localhost:3001
echo   ds2api:    http://localhost:5001
echo  ========================================
echo.
echo  Press any key to stop all services...
pause >nul

REM --- Kill all ---
taskkill /fi "WindowTitle eq ds2api" /t /f >nul 2>&1
taskkill /fi "WindowTitle eq nightcode-backend" /t /f >nul 2>&1
taskkill /fi "WindowTitle eq nightcode-frontend" /t /f >nul 2>&1
echo All services stopped.
