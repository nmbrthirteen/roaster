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
powershell -NoProfile -Command "$o=Get-CimInstance Win32_OperatingSystem; Write-Host ('    ' + $o.Caption + '  build ' + $o.BuildNumber); if ($o.OperatingSystemSKU -in 4,27,48,49,161,162) { Write-Host '    Assigned Access: available' } else { Write-Host '    Assigned Access: NOT available on this edition' }"
echo     arch %PROCESSOR_ARCHITECTURE%

echo.
echo   [Tools]
where go >nul 2>&1 && (for /f "tokens=3" %%v in ('go version') do echo     go %%v) || echo     go: MISSING
call :findsdk
if defined MAKEAPPX (echo     makeappx: %MAKEAPPX%) else (echo     makeappx: MISSING)
powershell -NoProfile -Command "$w = winget search --id Microsoft.WindowsSDK --source winget 2>$null; if ($w) { Write-Host '    winget sees these SDK packages:'; $w | Select-Object -Skip 2 | ForEach-Object { Write-Host ('      ' + $_) } }"

echo.
echo   [Build output]
if exist build\UpgamingRoaster.msix (echo     msix: built) else (echo     msix: not built)
if exist build\Upgaming.cer (echo     certificate: exported) else (echo     certificate: not exported)
if exist kiosk.exe (echo     kiosk.exe: built) else (echo     kiosk.exe: MISSING)
if exist roaster.exe (echo     roaster.exe: built) else (echo     roaster.exe: MISSING)

echo.
echo   [Installed package]
powershell -NoProfile -Command "$p = Get-AppxPackage -Name 'Upgaming.Roaster'; if (-not $p) { $p = Get-AppxPackage -AllUsers -Name 'Upgaming.Roaster' -ErrorAction SilentlyContinue | Select-Object -First 1 }; if ($p) { Write-Host ('    installed ' + $p.Version); Write-Host ('    identity  ' + $p.PackageFamilyName + '!Roaster') } else { Write-Host '    NOT installed' }; try { $v = Get-AppxProvisionedPackage -Online | Where-Object DisplayName -eq 'Upgaming.Roaster'; if ($v) { Write-Host '    on the device for every account' } else { Write-Host '    only for the account that installed it' } } catch { Write-Host '    cannot tell who has it, this window is not administrator' }"

echo.
echo   [Kiosk mode]
if exist "%ProgramFiles%\Roaster\kiosk.exe" (echo     installed to %ProgramFiles%\Roaster) else (echo     not installed to %ProgramFiles%\Roaster)
rem Get-AssignedAccess only reports store apps, so a kiosk set on kiosk.exe by
rem path shows up in the registry and nowhere it can see.
powershell -NoProfile -Command "$k='HKLM:\SOFTWARE\Microsoft\Windows\AssignedAccessConfiguration\Profiles'; if ((Test-Path $k) -and (Get-ChildItem $k -ErrorAction SilentlyContinue)) { Write-Host '    Assigned Access configuration: set' } else { Write-Host '    Assigned Access configuration: not set' }; try { $a = Get-AssignedAccess; if ($a) { $a | ForEach-Object { Write-Host ('    ' + $_.UserName + ' runs the store app ' + $_.AppUserModelId) } } } catch { Write-Host ('    cannot tell: ' + $_.Exception.Message) }"

echo.
echo   [Certificate trust]
powershell -NoProfile -Command "$c=Get-ChildItem Cert:\LocalMachine\TrustedPeople -ErrorAction SilentlyContinue | Where-Object { $_.Subject -eq 'CN=Upgaming' }; if ($c) { Write-Host '    trusted on this machine' } else { Write-Host '    not trusted yet' }"

echo.
echo   Copy everything above and send it over.
echo.
pause
exit /b 0

:findsdk
rem dir cannot match a folder in the middle of a path, so this searches bin for
rem the name rather than guessing at the version and architecture folders.
rem Tools fetched by package.bat count, since that is what it would use.
setlocal
set "M="
for %%r in ("%CD%\build\tools\bin" "%ProgramFiles(x86)%\Windows Kits\10\bin" "%ProgramFiles%\Windows Kits\10\bin") do (
  if exist "%%~r\" (
    for /f "delims=" %%f in ('dir /b /s "%%~r\makeappx.exe" 2^>nul') do set "M=%%f"
  )
)
endlocal & set "MAKEAPPX=%M%"
exit /b 0
