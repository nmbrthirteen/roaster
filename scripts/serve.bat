@echo off
rem Server supervisor. Restarts the roaster whenever it exits, for any reason.
rem Launched by kiosk.bat; not meant to be run directly.

cd /d "%~dp0.."
title Roaster server

:loop
echo [%date% %time%] starting >> roaster.log
roaster.exe
echo [%date% %time%] exited with %errorlevel%, restarting >> roaster.log

rem Keep the log from filling the disk over a multi-day event.
for %%A in (roaster.log) do if %%~zA GTR 5000000 (
  more +5000 roaster.log > roaster.log.tmp
  move /y roaster.log.tmp roaster.log >nul
)

timeout /t 3 /nobreak >nul
goto :loop
