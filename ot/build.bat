@echo off
cd /d "%~dp0"

echo [1/3] Building frontend...
cd front
call npm run build
if %errorlevel% neq 0 (
    echo Build failed.
    exit /b 1
)
cd ..

echo [2/3] Clearing embedded static folder...
if exist "cmd\dilmeterapi\static_v130_release" rd /s /q "cmd\dilmeterapi\static_v130_release"
mkdir "cmd\dilmeterapi\static_v130_release"

echo [3/3] Copying dist to static...
xcopy /e /y "front\dist\*" "cmd\dilmeterapi\static_v130_release\"

echo Done.
