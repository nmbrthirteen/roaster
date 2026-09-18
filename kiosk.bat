@echo off
rem Run the stand. Double click this at the event, or let autostart.bat run it
rem for you at boot.
rem
rem Nothing here is allowed to stay dead. The server restarts if it exits, the
rem browser restarts if it is closed, and the page reloads itself if the server
rem goes away underneath it.

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
go build -o roaster.exe .\cmd\roaster
if errorlevel 1 goto :failed
go build -ldflags="-H windowsgui" -o kiosk.exe .\cmd\kiosk
if errorlevel 1 goto :failed

echo   Starting the supervised server...
start "Roaster server" /min cmd /c "scripts\serve.bat"

echo   Waiting for it to answer...
for /l %%i in (1,1,30) do (
  curl -s -o nul http://localhost:3000/health && goto :ready
  timeout /t 1 /nobreak >nul
)

:ready
echo   Opening the kiosk. Close this window to stop everything.
echo.

:browserloop
kiosk.exe
echo   [%time%] browser closed, reopening
timeout /t 2 /nobreak >nul
goto :browserloop

:failed
echo   The build failed. The reason is above.
pause
exit /b 1
