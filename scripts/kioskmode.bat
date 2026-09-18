@echo off
rem Put this device into Windows kiosk mode on the packaged app. Take it out
rem again with:  kioskmode.bat off
rem
rem The picker in Settings only lists applications installed for the account
rem being locked down, and Add-AppxPackage installs for whoever ran it. That is
rem why doing this by hand so often ends with the app missing from the list.
rem This puts it on the device for every account, works out the identity kiosk
rem mode wants, and assigns it.

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

if /i "%~1"=="off" goto :off

echo.
echo   Kiosk mode
echo   ----------
echo.

if not exist "build\UpgamingRoaster.msix" (
  echo   The package is not built. Run scripts\package.bat first.
  goto :fail
)

echo   Putting the app on the device for every account...
powershell -NoProfile -ExecutionPolicy Bypass -Command "$ErrorActionPreference='Stop'; Add-AppxProvisionedPackage -Online -PackagePath 'build\UpgamingRoaster.msix' -SkipLicense | Out-Null; Write-Host '    provisioned'"
if errorlevel 1 goto :fail

set "ACCOUNT=%~1"
if defined ACCOUNT goto :haveaccount

echo.
echo   Accounts on this device:
powershell -NoProfile -Command "Get-LocalUser | Where-Object Enabled | ForEach-Object { Write-Host ('    ' + $_.Name) }"
echo.
set /p "ACCOUNT=  Which one is the stand? "

:haveaccount
if not defined ACCOUNT goto :fail

echo.
echo   Assigning the stand to %ACCOUNT%...
powershell -NoProfile -ExecutionPolicy Bypass -Command "$ErrorActionPreference='Stop'; $p = Get-AppxPackage -AllUsers -Name 'Upgaming.Roaster' | Select-Object -First 1; if (-not $p) { throw 'The package is not installed on this device. Run scripts\package.bat first.' }; $aumid = $p.PackageFamilyName + '!Roaster'; Write-Host ('    identity ' + $aumid); Set-AssignedAccess -AppUserModelId $aumid -UserName '%ACCOUNT%'; Write-Host '    assigned'"
if errorlevel 1 goto :refused

echo.
echo   Done. Sign out and sign in as %ACCOUNT% to see it.
echo   The account signs in to the stand and nothing else.
echo.
echo   To undo it:  scripts\kioskmode.bat off
echo.
pause
exit /b 0

:off
echo.
echo   Taking this device out of kiosk mode...
powershell -NoProfile -ExecutionPolicy Bypass -Command "$ErrorActionPreference='Stop'; Clear-AssignedAccess; Write-Host '    cleared'"
if errorlevel 1 goto :fail
echo.
echo   Done. The account signs in to Windows again.
echo.
pause
exit /b 0

:refused
echo.
echo   Windows would not take it. In the order worth checking:
echo.
echo     1. The account has never signed in, so the app is not installed for it
echo        yet. Sign in as %ACCOUNT% once, sign out, and run this again.
echo     2. The edition has no Assigned Access. Run scripts\check.bat; it says
echo        so on the first line.
echo     3. The edition has it but will not take a packaged desktop app, only a
echo        store app. Then the way to lock this device is
echo        scripts\lockdown.bat, which replaces the shell and needs no
echo        packaging at all.
echo.
pause
exit /b 1

:fail
echo.
echo   Kiosk mode was not set. The reason is above.
pause
exit /b 1
