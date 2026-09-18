@echo off
rem Make the stand come back by itself after a reboot or a power cut.
rem Run this once, as administrator.

setlocal
cd /d "%~dp0.."

set "TARGET=%cd%\kiosk.bat"
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
echo   Two more things the kiosk needs, in Windows settings:
echo     1. Sign-in options, turn on automatic sign-in for this account.
echo     2. Power and battery, screen and sleep, set both to Never.
echo.
pause
