@echo off
rem Package the stand as an MSIX so Windows offers it in
rem Settings, Accounts, Set up a kiosk.
rem
rem That picker lists Microsoft Edge and installed packaged apps and nothing
rem else, so a plain executable can never appear in it. Packaging is the only
rem route, and a package has to be signed, so this makes a certificate, trusts
rem it on this machine, and installs the result.

setlocal
cd /d "%~dp0.."
title Package the roaster

net session >nul 2>&1
if errorlevel 1 (
  echo   Asking for administrator...
  powershell -NoProfile -Command "Start-Process -Verb RunAs -FilePath '%~f0'"
  exit /b 0
)

echo.
echo   Packaging the roaster
echo   ---------------------
echo.

where go >nul 2>&1
if errorlevel 1 (
  echo   Go is not installed. Run run.bat first.
  goto :fail
)

call :findsdk
if not defined MAKEAPPX (
  echo   makeappx and signtool are missing. Fetching them...
  call :fetchtools || goto :fail
  call :findsdk
)
if not defined MAKEAPPX (
  echo   Still no makeappx. Install the SDK by hand if the download is blocked:
  echo     winget install --id Microsoft.WindowsSDK.10.0.26100
  goto :fail
)
echo   Using %MAKEAPPX%

echo   Building...
go build -o roaster.exe .\cmd\roaster || goto :fail
go build -ldflags="-s -w -H windowsgui" -o kiosk.exe .\cmd\kiosk || goto :fail

echo   Laying out the package...
if exist build\msix rmdir /s /q build\msix
mkdir build\msix >nul 2>&1

copy /y packaging\AppxManifest.xml build\msix\ >nul
xcopy /e /i /y /q packaging\images build\msix\images >nul
copy /y kiosk.exe   build\msix\ >nul
copy /y roaster.exe build\msix\ >nul
if exist roaster.json (copy /y roaster.json build\msix\ >nul) else (copy /y roaster.example.json build\msix\roaster.json >nul)
if exist events xcopy /e /i /y /q events build\msix\events >nul

echo   Packing...
"%MAKEAPPX%" pack /d build\msix /p build\UpgamingRoaster.msix /o || goto :fail

echo   Signing...
rem One PowerShell call, on one line. Batch does not treat ^ as an escape
rem inside quotes, so a caret here reaches PowerShell and breaks the command.
powershell -NoProfile -ExecutionPolicy Bypass -Command "$ErrorActionPreference='Stop'; $s='CN=Upgaming'; $c=Get-ChildItem Cert:\CurrentUser\My | Where-Object { $_.Subject -eq $s } | Select-Object -First 1; if (-not $c) { $c = New-SelfSignedCertificate -Type Custom -Subject $s -KeyUsage DigitalSignature -FriendlyName 'Upgaming Roaster kiosk' -CertStoreLocation 'Cert:\CurrentUser\My' -TextExtension @('2.5.29.37={text}1.3.6.1.5.5.7.3.3','2.5.29.19={text}') }; Export-Certificate -Cert $c -FilePath 'build\Upgaming.cer' | Out-Null; Import-Certificate -FilePath 'build\Upgaming.cer' -CertStoreLocation 'Cert:\LocalMachine\TrustedPeople' | Out-Null; [IO.File]::WriteAllText('build\thumbprint.txt', $c.Thumbprint); Write-Host ('    certificate ' + $c.Thumbprint)"
if errorlevel 1 goto :fail

set /p THUMB=<build\thumbprint.txt
"%SIGNTOOL%" sign /fd SHA256 /sha1 %THUMB% build\UpgamingRoaster.msix || goto :fail

echo   Installing...
powershell -NoProfile -ExecutionPolicy Bypass -Command "$ErrorActionPreference='Stop'; Add-AppxPackage -Path 'build\UpgamingRoaster.msix' -ForceUpdateFromAnyVersion"
if errorlevel 1 goto :fail

echo.
echo   Installed as Upgaming Roaster. It starts by itself at sign-in.
echo   Settings, Accounts, Other users, Set up a kiosk, and it is in the list.
echo.
echo   To put it on another device with no terminal, copy these two files:
echo     build\Upgaming.cer            double click, install to
echo                                   Local Machine, Trusted People
echo     build\UpgamingRoaster.msix    double click, Install
echo.
pause
exit /b 0

:findsdk
rem Tools fetched here win over an installed SDK, then either Program Files.
rem The architecture folder that matches the machine is preferred, with x64 as
rem the fallback, since it runs everywhere through emulation.
setlocal enabledelayedexpansion
set "M="
set "S="
set "ARCH=x64"
if /i "%PROCESSOR_ARCHITECTURE%"=="ARM64" set "ARCH=arm64"
for %%a in (x64 %ARCH%) do (
  for %%r in ("%CD%\build\tools" "%ProgramFiles(x86)%\Windows Kits\10" "%ProgramFiles%\Windows Kits\10") do (
    for /f "delims=" %%f in ('dir /b /s "%%~r\bin\*\%%a\makeappx.exe" 2^>nul') do set "M=%%f"
    for /f "delims=" %%f in ('dir /b /s "%%~r\bin\*\%%a\signtool.exe" 2^>nul') do set "S=%%f"
  )
)
endlocal & set "MAKEAPPX=%M%" & set "SIGNTOOL=%S%"
exit /b 0

:fetchtools
rem Microsoft ships makeappx and signtool in a 22MB package. Installing the
rem whole SDK for two executables is gigabytes for no reason.
powershell -NoProfile -ExecutionPolicy Bypass -Command "$ErrorActionPreference='Stop'; $ProgressPreference='SilentlyContinue'; New-Item 'build' -ItemType Directory -Force | Out-Null; Invoke-WebRequest 'https://www.nuget.org/api/v2/package/Microsoft.Windows.SDK.BuildTools' -OutFile 'build\sdktools.zip'; Remove-Item 'build\tools' -Recurse -Force -ErrorAction SilentlyContinue; Expand-Archive 'build\sdktools.zip' 'build\tools' -Force; Remove-Item 'build\sdktools.zip'; Write-Host '    fetched'"
exit /b %errorlevel%

:fail
echo.
echo   Packaging failed. The reason is above.
pause
exit /b 1
