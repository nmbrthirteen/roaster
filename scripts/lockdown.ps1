<#
  Locks this Windows device to the stand and keeps it up.

  The account logs in by itself and gets kiosk.exe instead of the desktop, so
  there is no taskbar, no start menu and nothing else to open. The screen never
  sleeps, updates never reboot under a visitor, and a crash comes straight back
  because Windows restarts the shell.

  Run scripts\lockdown.bat, which elevates and calls this. scripts\unlock.bat
  puts everything back.
#>
param(
  [Parameter(Mandatory = $true)][string]$User,
  [Parameter(Mandatory = $true)][string]$Sid,
  [switch]$Undo,
  # Time of a nightly reboot, "04:30". Left off, nothing reboots.
  [string]$NightlyReboot
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'location.ps1')
$root = Split-Path -Parent $PSScriptRoot
$account = $User.Split('\')[-1]
$exe = Join-Path $root 'kiosk.exe'
$task = 'Roaster nightly reboot'

if (-not (Get-PSDrive -Name HKU -ErrorAction SilentlyContinue)) {
  New-PSDrive -PSProvider Registry -Name HKU -Root HKEY_USERS -Scope Script | Out-Null
}

function Set-Value($path, $name, $value, $kind = 'DWord') {
  if (-not (Test-Path $path)) { New-Item -Path $path -Force | Out-Null }
  New-ItemProperty -Path $path -Name $name -Value $value -PropertyType $kind -Force | Out-Null
}

function Clear-Value($path, $name) {
  if (Test-Path $path) { Remove-ItemProperty -Path $path -Name $name -ErrorAction SilentlyContinue }
}

# powercfg and schtasks write to the error stream for things that are not
# failures here, such as deleting a task that was never created. A stopping
# preference would turn that into an exception, so they run quietly and their
# arguments are passed one by one, which keeps quoting out of it.
function Run([string]$command, [string[]]$arguments) {
  $was = $ErrorActionPreference
  $ErrorActionPreference = 'Continue'
  & $command @arguments 2>&1 | Out-Null
  $ErrorActionPreference = $was
}

$winlogon = "HKU:\$Sid\Software\Microsoft\Windows NT\CurrentVersion\Winlogon"
$policies = "HKU:\$Sid\Software\Microsoft\Windows\CurrentVersion\Policies\System"
$toasts = "HKU:\$Sid\Software\Microsoft\Windows\CurrentVersion\PushNotifications"
$logon = 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Winlogon'
$lockScreen = 'HKLM:\SOFTWARE\Policies\Microsoft\Windows\Personalization'
$update = 'HKLM:\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU'
$reporting = 'HKLM:\SOFTWARE\Microsoft\Windows\Windows Error Reporting'
$reliability = 'HKLM:\SOFTWARE\Policies\Microsoft\Windows NT\Reliability'
$userLocation = "HKU:\$Sid\Software\Microsoft\Windows\CurrentVersion\CapabilityAccessManager\ConsentStore\location"

if ($Undo) {
  Write-Host ''
  Write-Host '  Unlocking the device' -ForegroundColor Cyan

  Clear-Value $winlogon 'Shell'
  Clear-Value $policies 'DisableTaskMgr'
  Clear-Value $policies 'DisableLockWorkstation'
  Clear-Value $policies 'DisableChangePassword'
  Clear-Value $toasts 'ToastEnabled'
  Clear-Value $lockScreen 'NoLockScreen'
  Clear-Value $update 'NoAutoRebootWithLoggedOnUsers'
  Clear-Value $reliability 'ShutdownReasonUI'
  Clear-Value $reporting 'DontShowUI'
  Set-Value $logon 'AutoAdminLogon' '0' 'String'
  Clear-Value $logon 'DefaultPassword'
  Clear-Value $logon 'DefaultUserName'
  Clear-Value $logon 'DefaultDomainName'

  # Windows' own defaults: the screen off after ten minutes, asleep after
  # thirty, and USB ports allowed to suspend again.
  Run 'powercfg' @('/change', 'monitor-timeout-ac', '10')
  Run 'powercfg' @('/change', 'standby-timeout-ac', '30')
  Run 'powercfg' @('/change', 'monitor-timeout-dc', '5')
  Run 'powercfg' @('/change', 'standby-timeout-dc', '15')
  Run 'powercfg' @('/setacvalueindex', 'SCHEME_CURRENT', '2a737441-1930-4402-8d77-b2bebba308a3', '48e6b7a6-50f5-4782-a5d4-53bb8f07e226', '1')
  Run 'powercfg' @('/setactive', 'SCHEME_CURRENT')

  Run 'schtasks' @('/delete', '/tn', $task, '/f')

  Write-Host ''
  Write-Host '  Done. The desktop comes back at the next sign-in.'
  Write-Host '  Reboot now to see it.'
  Write-Host ''
  Read-Host '  Press Enter to close'
  exit 0
}

if (-not (Test-Path $exe)) {
  Write-Host ''
  Write-Host "  kiosk.exe is not built yet. Run run.bat first." -ForegroundColor Yellow
  Write-Host ''
  exit 1
}

# WebView2 is what draws the page. Windows 11 ships it and Edge keeps it
# updated, but a stripped image can be missing it, and that is a black screen.
$webview2 = @(
  'HKLM:\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}',
  'HKLM:\SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}'
) | Where-Object { Test-Path $_ }

Write-Host ''
Write-Host '  Locking the device to the stand' -ForegroundColor Cyan
Write-Host "  Account: $account"
Write-Host "  App:     $exe"
Write-Host ''

if (-not $webview2) {
  Write-Host '  The Microsoft Edge WebView2 Runtime is not installed on this device.' -ForegroundColor Yellow
  Write-Host '  Install it before locking, or the screen comes up black:'
  Write-Host '  https://developer.microsoft.com/microsoft-edge/webview2/'
  Write-Host ''
  $answer = Read-Host '  Carry on anyway? (y/N)'
  if ($answer -ne 'y') { exit 1 }
}

# The app replaces the desktop for this account. Nothing else starts, so there
# is nothing else to reach.
Set-Value $winlogon 'Shell' "`"$exe`" -shell" 'String'

# A shell that exits is started again by Windows, which is the watchdog for a
# crash at four in the morning.
Set-Value $logon 'AutoRestartShell' 1

# The account signs in by itself after a reboot or a power cut. A blank password
# needs nothing stored; Windows blocks blank-password accounts over the network.
Set-Value $logon 'AutoAdminLogon' '1' 'String'
Set-Value $logon 'DefaultUserName' $account 'String'
Set-Value $logon 'DefaultDomainName' $env:COMPUTERNAME 'String'

# What is left of Ctrl+Alt+Delete, which no program can take.
Set-Value $policies 'DisableTaskMgr' 1
Set-Value $policies 'DisableLockWorkstation' 1
Set-Value $policies 'DisableChangePassword' 1
Set-Value $lockScreen 'NoLockScreen' 1
Set-Value $toasts 'ToastEnabled' 0

# Nothing pops up over the stand: no update reboot, no crash report, no reason
# box after a power cut.
Set-Value $update 'NoAutoRebootWithLoggedOnUsers' 1
Set-Value $reporting 'DontShowUI' 1
Set-Value $reliability 'ShutdownReasonUI' 0

Enable-Location
try {
  Invoke-LocationPolicy
} catch {
  Write-Host "  could not force location on: $($_.Exception.Message)"
}
Set-Value "HKU:\$Sid\Software\Microsoft\TabletTip\1.7" 'EnableDesktopModeAutoInvoke' 0
Set-Value $userLocation 'Value' 'Allow' 'String'
Set-Value "$userLocation\NonPackaged" 'Value' 'Allow' 'String'

# Never off, never asleep, and the USB port stays powered so the printer does
# not vanish overnight.
Run 'powercfg' @('/change', 'monitor-timeout-ac', '0')
Run 'powercfg' @('/change', 'monitor-timeout-dc', '0')
Run 'powercfg' @('/change', 'standby-timeout-ac', '0')
Run 'powercfg' @('/change', 'standby-timeout-dc', '0')
Run 'powercfg' @('/change', 'disk-timeout-ac', '0')
Run 'powercfg' @('/change', 'hibernate-timeout-ac', '0')
Run 'powercfg' @('/hibernate', 'off')
Run 'powercfg' @('/setacvalueindex', 'SCHEME_CURRENT', '2a737441-1930-4402-8d77-b2bebba308a3', '48e6b7a6-50f5-4782-a5d4-53bb8f07e226', '0')
Run 'powercfg' @('/setactive', 'SCHEME_CURRENT')

# A stand that runs for days is steadier for a restart in the small hours, and
# it is the one thing here that has to be someone's decision.
if (-not $NightlyReboot) {
  $NightlyReboot = Read-Host '  Reboot nightly at what time? 04:30, or Enter for never'
}

if ($NightlyReboot) {
  Run 'schtasks' @('/create', '/tn', $task, '/tr', 'shutdown /r /t 0 /f',
    '/sc', 'daily', '/st', $NightlyReboot, '/ru', 'SYSTEM', '/f')
  Write-Host "  Rebooting nightly at $NightlyReboot."
} else {
  Run 'schtasks' @('/delete', '/tn', $task, '/f')
}

Write-Host ''
Write-Host '  Done. Reboot and the stand comes up by itself.' -ForegroundColor Green
Write-Host ''
Write-Host '  Ways back in:'
Write-Host '    The hidden menu in the app has Exit, which starts the desktop.'
Write-Host '    scripts\unlock.bat undoes all of this.'
Write-Host '    Holding Shift while signing in skips the automatic sign-in.'
Write-Host ''
Write-Host '  Two things this cannot set, both in the firmware:'
Write-Host '    Restore power state after a power cut, so the device boots itself.'
Write-Host '    Boot without a keyboard, if the firmware stops for one.'
Write-Host ''
Write-Host '  Automatic sign-in works as it stands only if this account has a blank'
Write-Host '  password. Windows will not store a real one except in clear text, so'
Write-Host '  either clear the password or use Sysinternals Autologon, which keeps it'
Write-Host '  in the LSA secret store instead.'
Write-Host ''
Read-Host '  Press Enter to close'
