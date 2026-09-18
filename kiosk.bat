@echo off
rem Run the stand. Double click this at the event.
rem
rem Same build as run.bat, then opens Edge locked to one full screen tab with
rem the touch gestures that would navigate away disabled.

setlocal
cd /d "%~dp0"
title Roaster kiosk

echo.
echo   Roaster kiosk
echo   -------------
echo.

where go >nul 2>&1
if errorlevel 1 (
  echo   Go is not installed. Run run.bat first.
  pause
  exit /b 1
)

if not exist "roaster.json" copy /y "roaster.example.json" "roaster.json" >nul

echo   Building...
go build -o roaster.exe .\cmd\roaster || goto :failed
go build -ldflags="-H windowsgui" -o kiosk.exe .\cmd\kiosk || goto :failed

rem The server runs in its own window so closing the browser does not kill it.
start "Roaster server" /min roaster.exe
echo   Waiting for the server...
timeout /t 3 /nobreak >nul

echo   Opening the kiosk.
kiosk.exe

echo.
echo   Kiosk closed. The server is still running in its own window.
pause
exit /b 0

:failed
echo   The build failed. The reason is above.
pause
exit /b 1
