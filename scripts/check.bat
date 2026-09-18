@echo off
rem Reports what state this machine is in, so a failure can be read rather than
rem guessed at. Safe to run any time; it changes nothing.

setlocal
cd /d "%~dp0.."
title Roaster check

echo.
echo   Roaster check
echo   -------------
echo.

echo   [Windows]
powershell -NoProfile -Command "$o=Get-CimInstance Win32_OperatingSystem; Write-Host ('    ' + $o.Caption + '  build ' + $o.BuildNumber)"
powershell -NoProfile -Command "$e=(Get-CimInstance Win32_OperatingSystem).OperatingSystemSKU; if ($e -in 4,27,48,49,161,162) { Write-Host '    Assigned Access: available (Pro or better)' } else { Write-Host '    Assigned Access: NOT available on this edition' }"
echo   [Architecture]
echo     %PROCESSOR_ARCHITECTURE%

echo.
echo   [Tools]
where go >nul 2>&1 && (for /f "tokens=3" %%v in ('go version') do echo     go %%v) || echo     go: MISSING
set "MAKEAPPX="
for /f "delims=" %%f in ('dir /b /s "%ProgramFiles(x86)%\Windows Kits\10\bin\*\x64\makeappx.exe" 2^>nul') do set "MAKEAPPX=%%f"
if defined MAKEAPPX (echo     makeappx: found) else (echo     makeappx: MISSING, run: winget install Microsoft.WindowsSDK)

echo.
echo   [Build output]
if exist build\UpgamingRoaster.msix (echo     msix: built) else (echo     msix: not built)
if exist build\Upgaming.cer (echo     certificate: exported) else (echo     certificate: not exported)
if exist kiosk.exe (echo     kiosk.exe: built) else (echo     kiosk.exe: MISSING)
if exist roaster.exe (echo     roaster.exe: built) else (echo     roaster.exe: MISSING)

echo.
echo   [Installed package]
powershell -NoProfile -Command "$p=Get-AppxPackage -Name 'Upgaming.Roaster'; if ($p) { Write-Host ('    installed ' + $p.Version + ' at ' + $p.InstallLocation) } else { Write-Host '    NOT installed' }"

echo.
echo   [Certificate trust]
powershell -NoProfile -Command "$c=Get-ChildItem Cert:\LocalMachine\TrustedPeople ^| Where-Object { $_.Subject -eq 'CN=Upgaming' }; if ($c) { Write-Host '    trusted on this machine' } else { Write-Host '    NOT trusted, the package will not install' }"

echo.
echo   [Sideloading]
powershell -NoProfile -Command "$k='HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock'; $v=(Get-ItemProperty $k -ErrorAction SilentlyContinue).AllowAllTrustedApps; if ($v -eq 1 -or $null -eq $v) { Write-Host '    allowed' } else { Write-Host '    BLOCKED, set AllowAllTrustedApps to 1' }"

echo.
echo   Copy everything above and send it over.
echo.
pause
