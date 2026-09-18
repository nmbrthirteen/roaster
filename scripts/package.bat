@echo off
rem Package the stand as an MSIX so Windows offers it in the kiosk picker.
rem Needs the Windows SDK: winget install Microsoft.WindowsSDK

net session >nul 2>&1
if errorlevel 1 (
  echo   Asking for administrator...
  powershell -NoProfile -Command "Start-Process -Verb RunAs -FilePath '%~f0'"
  exit /b 0
)

powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0package.ps1"
if errorlevel 1 (
  echo.
  echo   Packaging failed. The reason is above.
)
pause
