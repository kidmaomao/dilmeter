@echo off
cd /d "%~dp0"

echo [1/2] Building frontend...
cd front
call npm run build
if %errorlevel% neq 0 (
    echo Build failed.
    exit /b 1
)
cd ..

echo [2/2] Synchronizing current frontend resources...
powershell -NoProfile -ExecutionPolicy Bypass -File "tools\sync_embedded_frontend.ps1"
if %errorlevel% neq 0 exit /b 1

echo Done.
