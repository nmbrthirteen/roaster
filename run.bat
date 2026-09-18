@echo off
rem Build and run the roaster for testing. Double click this.
rem
rem Installs Go if it is missing, copies the headline font across from the
rem lifeat frontend if it is checked out next door, builds, and opens the kiosk
rem in your default browser.

setlocal
cd /d "%~dp0"
title Roaster

echo.
echo   Roaster
echo   -------
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
go build -ldflags="-H windowsgui" -o kiosk.exe .\cmd\kiosk
if errorlevel 1 goto :buildfailed

echo   Opening http://localhost:3000/kiosk
start "" /b cmd /c "timeout /t 2 /nobreak >nul & start "" http://localhost:3000/kiosk"
echo.
roaster.exe
goto :end
:needgo

if exist ".git" (
  echo   Pulling latest...
  git diff --quiet 2>nul && git pull --quiet || echo   Local changes present, skipping pull.
)

if not exist "roaster.json" copy /y "roaster.example.json" "roaster.json" >nul

echo   Building...
go build -o roaster.exe .\cmd\roaster
if errorlevel 1 goto :buildfailed
go build -ldflags="-H windowsgui" -o kiosk.exe .\cmd\kiosk
if errorlevel 1 goto :buildfailed

echo   Opening http://localhost:3000/kiosk
start "" /b cmd /c "timeout /t 2 /nobreak >nul & start "" http://localhost:3000/kiosk"
echo.
roaster.exe
goto :end


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

:end
echo.
echo   Roaster stopped. The reason is above and in roaster.log.
pause
