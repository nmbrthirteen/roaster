@echo off
rem Open the receipt designer. A window you can close, not a locked screen.
rem Use this to change the receipt layout, pick a printer, or add an event.

setlocal
cd /d "%~dp0"
if not exist "kiosk.exe" call :build || exit /b 1
start "" kiosk.exe -preview
exit /b 0

:build
echo Building...
go build -o roaster.exe .\cmd\roaster || exit /b 1
go build -ldflags="-s -w -H windowsgui" -o kiosk.exe .\cmd\kiosk || exit /b 1
exit /b 0
