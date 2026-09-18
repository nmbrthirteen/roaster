@echo off
rem Run the stand. Locked to one full screen tab, and it keeps itself alive.

setlocal
cd /d "%~dp0"
if not exist "kiosk.exe" call :build || exit /b 1
start "" kiosk.exe
exit /b 0

:build
echo Building...
go build -o roaster.exe .\cmd\roaster || exit /b 1
go build -ldflags="-s -w -H windowsgui" -o kiosk.exe .\cmd\kiosk || exit /b 1
exit /b 0
