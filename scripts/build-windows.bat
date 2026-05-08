@echo off
setlocal enabledelayedexpansion

set "OUTPUT=C:\Users\scw00\Downloads\stash-vr.exe"
set "SKIP_DIRTY="

:parse_args
if "%~1"=="" goto args_done
if /i "%~1"=="--skip-dirty" (
    set "SKIP_DIRTY=1"
    shift
    goto parse_args
)
if /i "%~1"=="--output" (
    set "OUTPUT=%~2"
    shift
    shift
    goto parse_args
)
echo Unknown argument: %~1
echo Usage: build-windows.bat [--output PATH] [--skip-dirty]
exit /b 2
:args_done

pushd "%~dp0.." || exit /b 1

set "GO_BIN="
where go >nul 2>&1
if not errorlevel 1 goto have_go

if exist "C:\Program Files\Go\bin\go.exe" set "GO_BIN=C:\Program Files\Go\bin"
if not defined GO_BIN if exist "C:\Go\bin\go.exe" set "GO_BIN=C:\Go\bin"
if not defined GO_BIN if exist "%LOCALAPPDATA%\Programs\Go\bin\go.exe" set "GO_BIN=%LOCALAPPDATA%\Programs\Go\bin"
if not defined GO_BIN if exist "%USERPROFILE%\go\bin\go.exe" set "GO_BIN=%USERPROFILE%\go\bin"

if not defined GO_BIN (
    echo ERROR: go.exe not found on PATH or in standard install locations.
    popd
    exit /b 1
)
set "PATH=%GO_BIN%;%PATH%"
:have_go

set "SHA="
for /f "usebackq delims=" %%i in (`git rev-parse --short HEAD 2^>nul`) do set "SHA=%%i"
if not defined SHA (
    echo ERROR: git rev-parse --short HEAD failed.
    popd
    exit /b 1
)

set "BRANCH="
for /f "usebackq delims=" %%i in (`git rev-parse --abbrev-ref HEAD 2^>nul`) do set "BRANCH=%%i"
if not defined BRANCH set "BRANCH=dev"
if /i "!BRANCH!"=="HEAD" set "BRANCH=dev"

if not defined SKIP_DIRTY (
    set "DIRTY="
    for /f "usebackq delims=" %%i in (`git status --porcelain`) do set "DIRTY=1"
    if defined DIRTY set "SHA=!SHA!-dirty"
)

set "VERSION=!BRANCH!"
set "LDFLAGS=-s -w -X stash-vr/internal/build.Version=!VERSION! -X stash-vr/internal/build.SHA=!SHA!"

for %%I in ("%OUTPUT%") do set "OUT_DIR=%%~dpI"
if not exist "%OUT_DIR%" mkdir "%OUT_DIR%"

set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64

echo Building stash-vr (!VERSION!+!SHA!) -^> %OUTPUT%
go build -trimpath -ldflags "!LDFLAGS!" -o "%OUTPUT%" ./cmd/stash-vr
if errorlevel 1 (
    echo ERROR: go build failed.
    popd
    exit /b 1
)

for %%F in ("%OUTPUT%") do set "SIZE_BYTES=%%~zF"
set /a "SIZE_KB=!SIZE_BYTES!/1024"
echo OK  %OUTPUT%  (!SIZE_KB! KB, version !VERSION!+!SHA!)

popd
endlocal
exit /b 0
