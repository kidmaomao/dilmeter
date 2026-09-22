@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0"

set "GO_EXE=go"
if exist "%~dp0..\.tools\go\bin\go.exe" set "GO_EXE=%~dp0..\.tools\go\bin\go.exe"
set "WINRES_EXE=go-winres"
if exist "%~dp0..\.cache\gopath\bin\go-winres.exe" set "WINRES_EXE=%~dp0..\.cache\gopath\bin\go-winres.exe"
set "APP_VERSION=1.5.0"
if defined DILMETER_APP_VERSION set "APP_VERSION=%DILMETER_APP_VERSION%"
set "BUILD_VARIANT=release"
if defined DILMETER_BUILD_VARIANT set "BUILD_VARIANT=%DILMETER_BUILD_VARIANT%"

echo [1/4] Generating DilmeterOT Windows resources...
"%WINRES_EXE%" make --in cmd\dilmeterapi\winres\winres.json --out cmd\dilmeterapi\rsrc
if errorlevel 1 exit /b 1

echo [2/4] Running DilmeterOT tests...
"%GO_EXE%" test -count=1 ./...
if errorlevel 1 exit /b 1

set "OUTPUT_EXE=DilmeterOT-v%APP_VERSION%.exe"
echo [3/4] Building DilmeterOT v%APP_VERSION%...
"%GO_EXE%" build -buildvcs=false -trimpath -ldflags="-H=windowsgui -s -w -X main.AppName=DilmeterOT -X main.AppVersion=%APP_VERSION% -X main.BuildVariant=%BUILD_VARIANT%" -o "%OUTPUT_EXE%" ./cmd/dilmeterapi/
if errorlevel 1 exit /b 1

echo [4/4] Applying DilmeterOT file metadata and icon...
pushd "cmd\dilmeterapi"
"%WINRES_EXE%" patch --in "winres\winres.json" --no-backup "..\..\%OUTPUT_EXE%"
set "OT_PATCH_RESULT=%ERRORLEVEL%"
popd
if not "%OT_PATCH_RESULT%"=="0" exit /b %OT_PATCH_RESULT%

echo Created %OUTPUT_EXE%
endlocal
