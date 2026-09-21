<#
  Puts this device into Windows kiosk mode on kiosk.exe, or takes it out.

  Assigned Access, as a restricted profile that starts kiosk.exe by itself
  and allows only what the stand runs. Neither the picker in Settings nor
  Set-AssignedAccess can express that: both take store apps only. The
  configuration goes in through the MDM bridge instead, and the bridge answers
  only to SYSTEM, so this runs itself a second time as SYSTEM for that one
  step. It needs Windows 11 22H2 or later.

  Run scripts\kioskmode.bat, which builds, elevates and calls this.
#>
param(
  [string]$Source,
  # A standard account to lock. Left off, Windows makes one and signs it in by
  # itself, which is what a stand wants.
  [string]$Account,
  [switch]$Off,
  # Set only on the SYSTEM run: where to write what happened.
  [string]$Result
)

$ErrorActionPreference = 'Stop'

# The path has to stay the same across builds, because the configuration names
# it. A packaged install lives in a folder named after its version.
$installDir = Join-Path $env:ProgramFiles 'Roaster'
$kioskPath = '%ProgramFiles%\Roaster\kiosk.exe'
$profileId = '{6F1C2B7E-3D4A-4E8B-9C21-5A7D0E3F9B14}'
$taskName = 'Roaster kiosk mode'

# After someone leaves the kiosk, the sign-in screen waits this long before the
# kiosk signs itself back in. Windows' 30 seconds is too short to tap an
# administrator's password in on a touch keyboard.
$logonUI = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Authentication\LogonUI'
$resumeAfterMs = 120000

# A swipe in from the edge of a touchscreen brings up the taskbar, and from
# there the power button. Off while the device is a stand, back on after.
$edgeUI = 'HKLM:\SOFTWARE\Policies\Microsoft\Windows\EdgeUI'

# Three- and four-finger swipes switch apps and show the desktop. They are a
# per-user setting with no policy behind them, and a kiosk account cannot open
# Settings to change its own, so the value goes into its registry from here:
# the account's own hive, and the default profile a fresh kiosk account is
# copied from.
function Disable-KioskGestures {
  $targets = @("$env:SystemDrive\Users\Default\NTUSER.DAT")
  $names = @(Get-LocalUser | Where-Object { $_.Name -like 'kioskUser*' -or $_.FullName -eq 'Upgaming Roaster' -or ($Account -and $_.Name -eq $Account) })
  foreach ($u in $names) {
    $loaded = "Registry::HKEY_USERS\$($u.SID.Value)"
    if (Test-Path $loaded) {
      Set-ItemProperty -Path "$loaded\Control Panel\Desktop" -Name 'TouchGestureSetting' -Value 0 -Type DWord
      continue
    }
    $userProfile = Get-CimInstance Win32_UserProfile | Where-Object { $_.SID -eq $u.SID.Value }
    if ($userProfile) { $targets += Join-Path $userProfile.LocalPath 'NTUSER.DAT' }
  }
  foreach ($hive in $targets) {
    if (-not (Test-Path $hive)) { continue }
    reg load 'HKU\RoasterKiosk' $hive | Out-Null
    if ($LASTEXITCODE -ne 0) { continue }
    try {
      reg add 'HKU\RoasterKiosk\Control Panel\Desktop' /v TouchGestureSetting /t REG_DWORD /d 0 /f | Out-Null
    } finally {
      [gc]::Collect()
      reg unload 'HKU\RoasterKiosk' | Out-Null
    }
  }
}

function Set-Configuration {
  $cim = Get-CimInstance -Namespace 'root\cimv2\mdm\dmmap' -ClassName 'MDM_AssignedAccess'
  if ($Off) {
    $cim.Configuration = $null
  } else {
    if ($Account) {
      $who = '<Account>' + [Security.SecurityElement]::Escape($Account) + '</Account>'
    } else {
      $who = '<AutoLogonAccount rs5:DisplayName="Upgaming Roaster" />'
    }
    # A restricted profile rather than a single-app kiosk: the single-app kind
    # runs only the program it names, and the stand is three. kiosk.exe starts
    # roaster.exe, and its window is WebView2, which runs as msedgewebview2.exe
    # from a folder named after its version, hence the wildcard. The menu also
    # runs netsh for Wi-Fi, shutdown for power, and PowerShell to list printers.
    # This kind also logs what it blocks, under AppLocker in Event Viewer.
    $xml = @"
<?xml version="1.0" encoding="utf-8"?>
<AssignedAccessConfiguration
    xmlns="http://schemas.microsoft.com/AssignedAccess/2017/config"
    xmlns:rs5="http://schemas.microsoft.com/AssignedAccess/201810/config"
    xmlns:v5="http://schemas.microsoft.com/AssignedAccess/2022/config">
  <Profiles>
    <Profile Id="$profileId">
      <AllAppsList>
        <AllowedApps>
          <App DesktopAppPath="$kioskPath" rs5:AutoLaunch="true" />
          <App DesktopAppPath="%ProgramFiles%\Roaster\roaster.exe" />
          <App DesktopAppPath="%ProgramFiles(x86)%\Microsoft\EdgeWebView\Application\*\msedgewebview2.exe" />
          <App DesktopAppPath="%ProgramFiles(x86)%\Microsoft\Edge\Application\*\msedgewebview2.exe" />
          <App DesktopAppPath="%windir%\System32\netsh.exe" />
          <App DesktopAppPath="%windir%\System32\shutdown.exe" />
          <App DesktopAppPath="%windir%\System32\WindowsPowerShell\v1.0\powershell.exe" />
        </AllowedApps>
      </AllAppsList>
      <rs5:FileExplorerNamespaceRestrictions>
      </rs5:FileExplorerNamespaceRestrictions>
      <v5:StartPins><![CDATA[{ "pinnedList": [] }]]></v5:StartPins>
      <Taskbar ShowTaskbar="false" />
    </Profile>
  </Profiles>
  <Configs>
    <Config>
      $who
      <DefaultProfile Id="$profileId" />
    </Config>
  </Configs>
</AssignedAccessConfiguration>
"@
    $cim.Configuration = [System.Net.WebUtility]::HtmlEncode($xml)
  }
  Set-CimInstance -CimInstance $cim
}

if ($Result) {
  try {
    Set-Configuration
    Set-Content -Path $Result -Value 'ok'
  } catch {
    Set-Content -Path $Result -Value $_.Exception.Message
  }
  exit 0
}

function Invoke-AsSystem {
  $resultFile = Join-Path $env:ProgramData 'Roaster\kioskmode.txt'
  New-Item -ItemType Directory -Force -Path (Split-Path $resultFile) | Out-Null
  Remove-Item $resultFile -ErrorAction SilentlyContinue

  $arguments = "-NoProfile -ExecutionPolicy Bypass -File `"$PSCommandPath`" -Result `"$resultFile`""
  if ($Off) { $arguments += ' -Off' }
  if ($Account) { $arguments += " -Account `"$Account`"" }

  $action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument $arguments
  $principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
  # A task's defaults refuse to start on battery, which left a laptop stand
  # waiting on a task that never ran.
  $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries `
    -ExecutionTimeLimit (New-TimeSpan -Minutes 10)
  Register-ScheduledTask -TaskName $taskName -Action $action -Principal $principal -Settings $settings -Force | Out-Null
  try {
    Start-ScheduledTask -TaskName $taskName
    # Creating the kiosk account and applying its policies can take Windows a
    # few minutes, so the wait is long and says it is still going.
    $started = Get-Date
    $deadline = $started.AddMinutes(5)
    $nextNote = $started.AddSeconds(15)
    while (-not (Test-Path $resultFile)) {
      if ((Get-Date) -gt $deadline) {
        $state = (Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue).State
        throw "Windows did not finish within 5 minutes (the task was $state). Run scripts\check.bat to see whether it applied anyway."
      }
      if ((Get-Date) -gt $nextNote) {
        Write-Host ("    still applying, {0} seconds so far..." -f [int]((Get-Date) - $started).TotalSeconds)
        $nextNote = (Get-Date).AddSeconds(15)
      }
      Start-Sleep -Milliseconds 500
    }
    # The file can exist a moment before its contents land.
    Start-Sleep -Milliseconds 300
    $answer = (Get-Content $resultFile -Raw).Trim()
  } finally {
    Unregister-ScheduledTask -TaskName $taskName -Confirm:$false -ErrorAction SilentlyContinue
    Remove-Item $resultFile -ErrorAction SilentlyContinue
  }
  if ($answer -ne 'ok') { throw $answer }
}

if ($Off) {
  Write-Host '  Taking this device out of kiosk mode...'
  Invoke-AsSystem
  Remove-ItemProperty -Path $logonUI -Name 'IdleTimeOut' -ErrorAction SilentlyContinue
  Remove-ItemProperty -Path $edgeUI -Name 'AllowEdgeSwipe' -ErrorAction SilentlyContinue
  Write-Host '    cleared'
  exit 0
}

if ($Account) {
  $user = Get-LocalUser -Name $Account -ErrorAction SilentlyContinue
  if (-not $user) { throw "There is no local account called $Account." }
  # By SID, because the group's name is translated on a non-English Windows.
  $admins = Get-LocalGroupMember -SID 'S-1-5-32-544' | Where-Object { $_.SID.Value -eq $user.SID.Value }
  if ($admins) { throw "$Account is an administrator. Kiosk mode takes a standard account only." }
}

Write-Host "  Installing to $installDir..."
# A running copy holds its executables open, and the copy over them would fail.
Get-Process -Name 'kiosk', 'roaster' -ErrorAction SilentlyContinue |
  Where-Object { $_.Path -and $_.Path.StartsWith($installDir, [StringComparison]::OrdinalIgnoreCase) } |
  Stop-Process -Force
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
Copy-Item (Join-Path $Source 'kiosk.exe'), (Join-Path $Source 'roaster.exe') $installDir -Force
# The stand keeps its settings here and the hidden menu saves to this file, so
# a second run to ship a new build must not put the old ones back.
$installed = Join-Path $installDir 'roaster.json'
if (-not (Test-Path $installed)) {
  $settings = Join-Path $Source 'roaster.json'
  if (-not (Test-Path $settings)) { $settings = Join-Path $Source 'roaster.example.json' }
  Copy-Item $settings $installed
}
$events = Join-Path $Source 'events'
if (Test-Path $events) { Copy-Item $events $installDir -Recurse -Force }

# The kiosk account otherwise cannot write to Program Files. Granting Modify
# here is what lets the hidden menu update the stand in place: it downloads a
# new build and swaps it in without an operator ever unlocking the device.
# By SID, because the group's name is translated on a non-English Windows.
icacls $installDir /grant '*S-1-5-32-545:(OI)(CI)M' /T /Q | Out-Null
if ($LASTEXITCODE -ne 0) { throw "icacls could not grant Users write access to $installDir." }
Write-Host '    installed'

if ($Account) {
  Write-Host "  Locking $Account to the stand..."
} else {
  Write-Host '  Locking the device to the stand, on an account Windows signs in by itself...'
}
Invoke-AsSystem
New-ItemProperty -Path $logonUI -Name 'IdleTimeOut' -PropertyType DWord -Value $resumeAfterMs -Force | Out-Null
# New-Item -Force would recreate an existing key and lose its other values.
if (-not (Test-Path $edgeUI)) { New-Item -Path $edgeUI | Out-Null }
New-ItemProperty -Path $edgeUI -Name 'AllowEdgeSwipe' -PropertyType DWord -Value 0 -Force | Out-Null
# The lock is already in place, so a failure here is worth a warning, not a stop.
try {
  Disable-KioskGestures
} catch {
  Write-Host "    could not turn off multi-finger gestures: $($_.Exception.Message)"
}
Write-Host '    assigned'

$packaged = Get-AppxPackage -AllUsers -Name 'Upgaming.Roaster' -ErrorAction SilentlyContinue
if ($packaged) {
  Write-Host ''
  Write-Host '  The packaged app is also on this device, and it starts itself at sign-in.'
  Write-Host '  Two copies race for the screen. Remove it in Settings, Apps, Installed apps.'
}
