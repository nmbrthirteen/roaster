@echo off
rem Launcher for the roaster kiosk.
rem
rem Double clicking the exe directly gives you a console window that closes on
rem any error before it can be read. This picks the build that matches the chip
rem and keeps the window open so a failure is visible.

cd /d "%~dp0"

if /i "%PROCESSOR_ARCHITECTURE%"=="ARM64" (
  set EXE=roaster-arm64.exe
) else (
  set EXE=roaster-amd64.exe
)

echo Starting %EXE% ...
echo Open http://localhost:3000 once it says listening.
echo.
"%EXE%"

echo.
echo Roaster stopped. The reason is above and in roaster.log.
pause
