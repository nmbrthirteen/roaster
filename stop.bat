@echo off
rem Stop everything, for when closing the window is not enough.

taskkill /f /im kiosk.exe >nul 2>&1
taskkill /f /im roaster.exe >nul 2>&1
echo Stopped.
