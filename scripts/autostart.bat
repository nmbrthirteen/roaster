@echo off
rem Make the stand come back by itself after a reboot or a power cut.
rem Run this once, as administrator.

setlocal
cd /d "%~dp0.."

set "TARGET=%cd%\kiosk.exe"
if not exist "kiosk.exe" (
  echo   kiosk.exe is not built yet. Run run.bat first.
  pause
  exit /b 1
)

set "LINK=%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup\Roaster.lnk"

powershell -NoProfile -Command ^
  "$s = (New-Object -ComObject WScript.Shell).CreateShortcut('%LINK%');" ^
  "$s.TargetPath = '%TARGET%';" ^
  "$s.WorkingDirectory = '%cd%';" ^
  "$s.WindowStyle = 7;" ^
  "$s.Save()"

if errorlevel 1 (
  echo   Could not create the startup shortcut.
  pause
  exit /b 1
)

echo.
echo   Roaster will now start automatically when this account logs in.
echo   Remove it by deleting:
echo   %LINK%
echo.
echo   This leaves Windows otherwise as it is: the desktop is still there behind
echo   the stand, and the machine can still sign out and sleep.
echo.
echo   For a device that is only ever the stand, run scripts\lockdown.bat
echo   instead. It replaces the desktop with the app, signs in by itself and
echo   stops the screen sleeping.
echo.
pause
