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
  echo   Still no makeappx. This is what came down:
  call :whatwegot
  echo.
  echo   Install the SDK by hand if the download is blocked:
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

rem Every build gets a version of its own. Windows compares versions to decide
rem whether an install is an update, so two packages that both say 1.0.0.0 are
rem the same package to it, and the second one is refused as already installed.
rem The parts are days since 2024 and minutes since midnight, which rise, stay
rem inside the 65535 a version part allows, and need nothing kept between runs.
powershell -NoProfile -ExecutionPolicy Bypass -Command "$d=[int][math]::Floor(((Get-Date) - [datetime]'2024-01-01').TotalDays); $m=[int][math]::Floor((Get-Date).TimeOfDay.TotalMinutes); Set-Content 'build\version.txt' ('1.0.' + $d + '.' + $m) -NoNewline"
if errorlevel 1 goto :fail
set /p VERSION=<build\version.txt
echo   Version %VERSION%
powershell -NoProfile -ExecutionPolicy Bypass -Command "$ErrorActionPreference='Stop'; $x=[xml](Get-Content 'packaging\AppxManifest.xml'); $x.Package.Identity.Version='%VERSION%'; $x.Save((Get-Item 'build\msix').FullName + '\AppxManifest.xml')"
if errorlevel 1 goto :fail
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
rem ForceApplicationShutdown closes the running stand, which is otherwise what
rem holds the old copy in place. Taking the old one out and starting again is
rem the way through anything else: a changed certificate, a half-installed
rem package, an identity that moved.
powershell -NoProfile -ExecutionPolicy Bypass -Command "$ErrorActionPreference='Stop'; try { Add-AppxPackage -Path 'build\UpgamingRoaster.msix' -ForceUpdateFromAnyVersion -ForceApplicationShutdown } catch { Write-Host ('    ' + $_.Exception.Message); Write-Host '    Removing the installed copy and trying once more...'; Get-AppxPackage -Name 'Upgaming.Roaster' | Remove-AppxPackage; Add-AppxPackage -Path 'build\UpgamingRoaster.msix' }"
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
rem
rem Both layouts keep the executables in bin\<version>\<architecture>\, and dir
rem cannot match a folder in the middle of a path: bin\*\x64\makeappx.exe looks
rem right and matches nothing, ever. So this searches bin for the name and
rem settles the architecture afterwards.
setlocal
set "ARCH=x64"
if /i "%PROCESSOR_ARCHITECTURE%"=="ARM64" set "ARCH=arm64"
set "M="
set "S="
for %%r in ("%CD%\build\tools\bin" "%ProgramFiles(x86)%\Windows Kits\10\bin" "%ProgramFiles%\Windows Kits\10\bin") do (
  if exist "%%~r\" (
    for /f "delims=" %%f in ('dir /b /s "%%~r\makeappx.exe" 2^>nul') do call :prefer M "%%f"
    for /f "delims=" %%f in ('dir /b /s "%%~r\signtool.exe" 2^>nul') do call :prefer S "%%f"
  )
)
endlocal & set "MAKEAPPX=%M%" & set "SIGNTOOL=%S%"
exit /b 0

:prefer
rem The first match is kept, and one built for this machine replaces it.
if not defined %~1 (
  set "%~1=%~2"
  exit /b 0
)
echo "%~2" | find /i "\%ARCH%\" >nul
if not errorlevel 1 set "%~1=%~2"
exit /b 0

:whatwegot
rem Read when the tools are still missing, so the reason is on screen rather
rem than guessed at.
if not exist "build\tools\" (
  echo     build\tools is not there, so the download did not land.
  exit /b 0
)
dir /b /s "build\tools\makeappx.exe" 2>nul
dir /b /s "build\tools\signtool.exe" 2>nul
echo     Nothing listed above means the package came down without them.
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
