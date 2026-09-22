@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0"

set "GO_EXE=go"
if exist "%~dp0..\.tools\go\bin\go.exe" set "GO_EXE=%~dp0..\.tools\go\bin\go.exe"
if "%GO_EXE%"=="go" (
    where go >nul 2>nul
    if errorlevel 1 (
        echo [ERROR] Go was not found.
        exit /b 1
    )
)

set "WINRES_EXE=go-winres"
if exist "%~dp0..\.cache\gopath\bin\go-winres.exe" set "WINRES_EXE=%~dp0..\.cache\gopath\bin\go-winres.exe"
if "%WINRES_EXE%"=="go-winres" (
    where go-winres >nul 2>nul
    if errorlevel 1 (
        echo [ERROR] go-winres was not found.
        exit /b 1
    )
)

set "APP_VERSION=1.5.0"
if defined DILMETER_APP_VERSION set "APP_VERSION=%DILMETER_APP_VERSION%"
set "BUILD_VARIANT=release"
if defined DILMETER_BUILD_VARIANT set "BUILD_VARIANT=%DILMETER_BUILD_VARIANT%"

echo [1/4] Generating DilmeterRT Windows resources...
pushd "cmd\dilmeterapi"
"%WINRES_EXE%" make --in "winres-rt\winres.json" --out rsrc
set "RT_RESULT=%ERRORLEVEL%"
popd
if not "%RT_RESULT%"=="0" exit /b %RT_RESULT%

echo [2/4] Running DilmeterRT tests...
"%GO_EXE%" test -tags dilmeter_rt -count=1 ./...
if errorlevel 1 exit /b 1

set "OUTPUT_EXE=DilmeterRT-v%APP_VERSION%.exe"
echo [3/4] Building DilmeterRT v%APP_VERSION%...
"%GO_EXE%" build -tags dilmeter_rt -buildvcs=false -trimpath -ldflags="-H=windowsgui -s -w -X main.AppName=DilmeterRT -X main.AppVersion=%APP_VERSION% -X main.BuildVariant=%BUILD_VARIANT%" -o "%OUTPUT_EXE%" ./cmd/dilmeterapi/
if errorlevel 1 exit /b 1

echo [4/4] Applying DilmeterRT file metadata and icon...
pushd "cmd\dilmeterapi"
"%WINRES_EXE%" patch --in "winres-rt\winres.json" --no-backup "..\..\%OUTPUT_EXE%"
set "RT_PATCH_RESULT=%ERRORLEVEL%"
popd
if not "%RT_PATCH_RESULT%"=="0" exit /b %RT_PATCH_RESULT%

echo Created %OUTPUT_EXE%
endlocal
