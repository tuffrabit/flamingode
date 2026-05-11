@echo off
setlocal enabledelayedexpansion

set APP_NAME=flamingode
set OUTPUT_DIR=bin
set LDFLAGS=-s -w
set VERSION=%-1

if defined VERSION set LDFLAGS=%LDFLAGS% -X github.com/tuffrabit/flamingode/internal/version.Version=%VERSION%

if exist "%OUTPUT_DIR%" rmdir /s /q "%OUTPUT_DIR%"
mkdir "%OUTPUT_DIR%"

for %%O in (linux darwin windows) do (
    for %%A in (amd64 arm64) do (
        if defined VERSION (
            set "OUTPUT=%OUTPUT_DIR%\%APP_NAME%-!VERSION!-%%O-%%A"
        ) else (
            set "OUTPUT=%OUTPUT_DIR%\%APP_NAME%-%%O-%%A"
        )
        if "%%O"=="windows" set "OUTPUT=!OUTPUT!.exe"

        echo Building %%O/%%A...
        set GOOS=%%O
        set GOARCH=%%A
        go build -ldflags "%LDFLAGS%" -o "!OUTPUT!" .
    )
)

echo Done. Binaries in %OUTPUT_DIR%\
