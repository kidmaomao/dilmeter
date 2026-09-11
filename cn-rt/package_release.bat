@echo off
setlocal
cd /d "%~dp0"

powershell -NoProfile -ExecutionPolicy Bypass -File "tools\package_release.ps1"
set "PACKAGE_RESULT=%ERRORLEVEL%"
if not "%PACKAGE_RESULT%"=="0" exit /b %PACKAGE_RESULT%

echo Created DilmeterCN/RT ZIP files and GitHub update manifests.
endlocal
