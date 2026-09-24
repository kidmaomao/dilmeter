@echo off
setlocal
cd /d "%~dp0"

set "GO_EXE=go"
if exist "%~dp0..\.tools\go\bin\go.exe" set "GO_EXE=%~dp0..\.tools\go\bin\go.exe"
set "WINRES_EXE=go-winres"
if exist "%~dp0..\.cache\gopath\bin\go-winres.exe" set "WINRES_EXE=%~dp0..\.cache\gopath\bin\go-winres.exe"
set "APP_VERSION=1.5.1"
if defined DILMETER_APP_VERSION set "APP_VERSION=%DILMETER_APP_VERSION%"
set "BUILD_VARIANT=release"
if defined DILMETER_BUILD_VARIANT set "BUILD_VARIANT=%DILMETER_BUILD_VARIANT%"

echo [1/3] Generating DilmeterCN Windows resources...
"%WINRES_EXE%" make --in cmd\dilmeterapi\winres\winres.json --out cmd\dilmeterapi\rsrc
if %errorlevel% neq 0 (
    echo Resource generation failed.
    exit /b 1
)

echo [2/3] Building desktop application...
set "OUTPUT_EXE=DilmeterCN-v%APP_VERSION%.exe"
"%GO_EXE%" build -buildvcs=false -trimpath -ldflags="-H=windowsgui -s -w -X main.AppVersion=%APP_VERSION% -X main.BuildVariant=%BUILD_VARIANT%" -o "%OUTPUT_EXE%" ./cmd/dilmeterapi/
if %errorlevel% neq 0 (
    echo Build failed.
    exit /b 1
)

echo [3/3] Applying DilmeterCN file metadata and icon...
pushd "cmd\dilmeterapi"
"%WINRES_EXE%" patch --in "winres\winres.json" --no-backup "..\..\%OUTPUT_EXE%"
set "CN_PATCH_RESULT=%ERRORLEVEL%"
popd
if not "%CN_PATCH_RESULT%"=="0" exit /b %CN_PATCH_RESULT%

echo Done.
endlocal
