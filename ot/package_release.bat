@echo off
setlocal
cd /d "%~dp0"

powershell -NoProfile -ExecutionPolicy Bypass -File "tools\package_release.ps1"
set "PACKAGE_RESULT=%ERRORLEVEL%"
if not "%PACKAGE_RESULT%"=="0" exit /b %PACKAGE_RESULT%

echo Created DilmeterOT ZIP file and GitHub update manifest.
endlocal
