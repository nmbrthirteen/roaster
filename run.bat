@echo off
rem One-time setup. Installs Go if it is missing, builds, and starts the stand.
rem After this, kiosk.exe is the only thing you need to open.

setlocal
cd /d "%~dp0"
title Roaster setup

echo.
echo   Roaster setup
echo   -------------
echo.

where go >nul 2>&1
if errorlevel 1 goto :needgo

if exist ".git" (
  echo   Pulling latest...
  git diff --quiet 2>nul && git pull --quiet || echo   Local changes present, skipping pull.
)

if not exist "roaster.json" copy /y "roaster.example.json" "roaster.json" >nul

echo   Building...
go build -o roaster.exe .\cmd\roaster
if errorlevel 1 goto :buildfailed
go build -ldflags="-s -w -H windowsgui" -o kiosk.exe .\cmd\kiosk
if errorlevel 1 goto :buildfailed

echo.
echo   Done. Starting the stand.
echo   From now on just open kiosk.exe, or run scripts\autostart.bat once so it
echo   starts by itself after a reboot.
echo.
start "" kiosk.exe
exit /b 0

:needgo
echo   Go is not installed. Installing it now...
echo.
winget install --id GoLang.Go -e --accept-source-agreements --accept-package-agreements
echo.
echo   Go is installed. Close this window, open a new one, and run this again.
echo   A new window is needed so Windows picks up the changed path.
pause
exit /b 1

:buildfailed
echo.
echo   The build failed. The reason is above.
pause
exit /b 1
