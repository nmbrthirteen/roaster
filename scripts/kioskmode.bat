@echo off
rem Put this device into Windows kiosk mode on the stand. Take it out again
rem with:  kioskmode.bat off
rem
rem   kioskmode.bat            Windows makes a kiosk account and signs it in
rem                            by itself after every restart.
rem   kioskmode.bat Stand      Locks an existing standard account instead.
rem
rem Installs kiosk.exe to Program Files and names it in the Assigned Access
rem configuration. That runs a desktop application as the kiosk on Windows 11
rem Pro and up, with no package, which the picker in Settings never offers.

setlocal
cd /d "%~dp0.."
title Kiosk mode

net session >nul 2>&1
if errorlevel 1 (
  echo   Asking for administrator...
  if "%~1"=="" powershell -NoProfile -Command "Start-Process -Verb RunAs -FilePath '%~f0'"
  if not "%~1"=="" powershell -NoProfile -Command "Start-Process -Verb RunAs -FilePath '%~f0' -ArgumentList '%~1'"
  exit /b 0
)

echo.
echo   Kiosk mode
echo   ----------
echo.

if /i "%~1"=="off" goto :off

set "FETCH="
where go >nul 2>&1
if errorlevel 1 (
  set "FETCH=-Download"
  goto :install
)
echo   Building...
go build -o roaster.exe .\cmd\roaster || goto :fail
go build -ldflags="-s -w -H windowsgui" -o kiosk.exe .\cmd\kiosk || goto :fail

:install
if "%~1"=="" (
  powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0kioskmode.ps1" -Source "%CD%" %FETCH%
) else (
  powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0kioskmode.ps1" -Source "%CD%" -Account "%~1" %FETCH%
)
if errorlevel 2 goto :fail
if errorlevel 1 goto :refused

echo.
echo   Done. Restart the device and it comes up on the stand.
echo   Ctrl+Alt+Del leaves it, for whoever holds the keyboard.
echo.
echo   To undo it:  scripts\kioskmode.bat off
echo.
pause
exit /b 0

:off
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0kioskmode.ps1" -Off
if errorlevel 1 goto :fail
echo.
echo   Done. Restart and the device signs in to Windows again.
echo.
pause
exit /b 0

:refused
echo.
echo   Windows would not take it. In the order worth checking:
echo.
echo     1. The edition has no Assigned Access. Run scripts\check.bat; it says
echo        so on the first line. Home cannot do this.
echo     2. Windows is older than Windows 11 21H2, which cannot run a desktop
echo        application as the kiosk. scripts\lockdown.bat works on anything.
echo.
pause
exit /b 1

:fail
echo.
echo   Kiosk mode was not changed. The reason is above.
pause
exit /b 1
