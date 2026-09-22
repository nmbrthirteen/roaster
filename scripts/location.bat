@echo off
rem Turns location on for this device. Windows 11 lists Wi-Fi networks only
rem with location on, and the stand's hidden menu scans for them.

setlocal
net session >nul 2>&1
if errorlevel 1 (
  echo   Asking for administrator...
  powershell -NoProfile -Command "Start-Process -Verb RunAs -FilePath '%~f0'"
  exit /b 0
)

echo.
echo   Turning location on
echo   -------------------
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0location.ps1" -Run
if errorlevel 1 (
  echo.
  echo   Location was not changed. The reason is above.
  pause
  exit /b 1
)
echo.
echo   Done. Restart the stand, then scan for networks again.
echo.
pause
