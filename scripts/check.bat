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
powershell -NoProfile -Command "$p=Get-AppxPackage -Name 'Upgaming.Roaster'; if ($p) { Write-Host ('    installed ' + $p.Version) } else { Write-Host '    NOT installed' }"

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
