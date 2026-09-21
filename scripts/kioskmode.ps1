<#
  Puts this device into Windows kiosk mode on kiosk.exe, or takes it out.

  Assigned Access runs a desktop application as the kiosk when its
  configuration names the executable by path, which Windows 11 has accepted
  since 21H2. Neither the picker in Settings nor Set-AssignedAccess can say
  that: both take store apps only. The configuration goes in through the MDM
  bridge instead, and the bridge answers only to SYSTEM, so this runs itself a
  second time as SYSTEM for that one step.

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
    $xml = @"
<?xml version="1.0" encoding="utf-8"?>
<AssignedAccessConfiguration
    xmlns="http://schemas.microsoft.com/AssignedAccess/2017/config"
    xmlns:rs5="http://schemas.microsoft.com/AssignedAccess/201810/config"
    xmlns:v4="http://schemas.microsoft.com/AssignedAccess/2021/config">
  <Profiles>
    <Profile Id="$profileId">
      <KioskModeApp v4:ClassicAppPath="$kioskPath" />
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
  Register-ScheduledTask -TaskName $taskName -Action $action -Principal $principal -Force | Out-Null
  try {
    Start-ScheduledTask -TaskName $taskName
    $deadline = (Get-Date).AddSeconds(90)
    while (-not (Test-Path $resultFile)) {
      if ((Get-Date) -gt $deadline) { throw 'Windows took more than 90 seconds to apply it, so it was stopped.' }
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
Write-Host '    assigned'

$packaged = Get-AppxPackage -AllUsers -Name 'Upgaming.Roaster' -ErrorAction SilentlyContinue
if ($packaged) {
  Write-Host ''
  Write-Host '  The packaged app is also on this device, and it starts itself at sign-in.'
  Write-Host '  Two copies race for the screen. Remove it in Settings, Apps, Installed apps.'
}
