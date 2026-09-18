@echo off
rem Put Windows back: the desktop, the lock screen, the power settings.

setlocal
for /f "tokens=1,2 delims=," %%a in ('whoami /user /fo csv /nh') do (
  set "WHO=%%~a"
  set "SID=%%~b"
)

net session >nul 2>&1
if errorlevel 1 (
  echo   Asking for administrator...
  powershell -NoProfile -Command ^
    "Start-Process -Verb RunAs -FilePath powershell -ArgumentList '-NoProfile','-ExecutionPolicy','Bypass','-File','%~dp0lockdown.ps1','-Undo','-User','%WHO%','-Sid','%SID%'"
  exit /b 0
)

powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0lockdown.ps1" -Undo -User "%WHO%" -Sid "%SID%"
