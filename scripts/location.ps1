param([switch]$Run, [string]$PolicyResult)

function Enable-Location {
  $policy = 'HKLM:\SOFTWARE\Policies\Microsoft\Windows\LocationAndSensors'
  if (Test-Path $policy) {
    foreach ($name in 'DisableLocation', 'DisableWindowsLocationProvider', 'DisableLocationScripting') {
      Remove-ItemProperty -Path $policy -Name $name -ErrorAction SilentlyContinue
    }
  }
  $privacy = 'HKLM:\SOFTWARE\Policies\Microsoft\Windows\AppPrivacy'
  if (Test-Path $privacy) { Remove-ItemProperty -Path $privacy -Name 'LetAppsAccessLocation' -ErrorAction SilentlyContinue }
  $consent = 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\CapabilityAccessManager\ConsentStore\location'
  if (-not (Test-Path $consent)) { New-Item -Path $consent -Force | Out-Null }
  New-ItemProperty -Path $consent -Name 'Value' -Value 'Allow' -PropertyType String -Force | Out-Null
  $service = Get-Service -Name 'lfsvc' -ErrorAction SilentlyContinue
  if ($service -and $service.StartType -eq 'Disabled') { Set-Service -Name 'lfsvc' -StartupType Manual }
}

function Enable-UserLocation {
  $keys = @(
    'Software\Microsoft\Windows\CurrentVersion\CapabilityAccessManager\ConsentStore\location',
    'Software\Microsoft\Windows\CurrentVersion\CapabilityAccessManager\ConsentStore\location\NonPackaged'
  )
  $write = {
    param($root)
    foreach ($key in $keys) { reg add "$root\$key" /v Value /t REG_SZ /d Allow /f | Out-Null }
  }
  $hives = @("$env:SystemDrive\Users\Default\NTUSER.DAT")
  foreach ($p in Get-CimInstance Win32_UserProfile | Where-Object { -not $_.Special -and $_.SID -like 'S-1-5-21-*' }) {
    if (Test-Path "Registry::HKEY_USERS\$($p.SID)") {
      & $write "HKU\$($p.SID)"
    } else {
      $hives += Join-Path $p.LocalPath 'NTUSER.DAT'
    }
  }
  $done = 0
  foreach ($hive in $hives) {
    if (-not (Test-Path $hive)) { continue }
    reg load 'HKU\RoasterLocation' $hive | Out-Null
    if ($LASTEXITCODE -ne 0) { continue }
    try {
      & $write 'HKU\RoasterLocation'
      $done++
    } finally {
      [gc]::Collect()
      reg unload 'HKU\RoasterLocation' | Out-Null
    }
  }
  $done
}

function Set-LocationPolicy([int]$Value = 2) {
  $namespace = 'root\cimv2\mdm\dmmap'
  $class = 'MDM_Policy_Config01_System02'
  $filter = "ParentID='./Vendor/MSFT/Policy/Config' and InstanceID='System'"
  $existing = Get-CimInstance -Namespace $namespace -ClassName $class -Filter $filter -ErrorAction SilentlyContinue
  if ($existing) {
    $existing.AllowLocation = $Value
    Set-CimInstance -CimInstance $existing
  } else {
    New-CimInstance -Namespace $namespace -ClassName $class -Property @{
      ParentID      = './Vendor/MSFT/Policy/Config'
      InstanceID    = 'System'
      AllowLocation = $Value
    } | Out-Null
  }
}

# The MDM bridge answers only to SYSTEM, so this file runs itself once more as
# SYSTEM through a scheduled task.
function Invoke-LocationPolicy {
  $task = 'Roaster location'
  $resultFile = Join-Path $env:ProgramData 'Roaster\location.txt'
  New-Item -ItemType Directory -Force -Path (Split-Path $resultFile) | Out-Null
  Remove-Item $resultFile -ErrorAction SilentlyContinue
  $script = Join-Path $PSScriptRoot 'location.ps1'
  $action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument "-NoProfile -ExecutionPolicy Bypass -File `"$script`" -PolicyResult `"$resultFile`""
  $principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
  $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit (New-TimeSpan -Minutes 5)
  Register-ScheduledTask -TaskName $task -Action $action -Principal $principal -Settings $settings -Force | Out-Null
  try {
    Start-ScheduledTask -TaskName $task
    $deadline = (Get-Date).AddMinutes(2)
    while (-not (Test-Path $resultFile)) {
      if ((Get-Date) -gt $deadline) { throw 'Windows did not apply the location policy within 2 minutes.' }
      Start-Sleep -Milliseconds 500
    }
    Start-Sleep -Milliseconds 300
    $answer = (Get-Content $resultFile -Raw).Trim()
  } finally {
    Unregister-ScheduledTask -TaskName $task -Confirm:$false -ErrorAction SilentlyContinue
    Remove-Item $resultFile -ErrorAction SilentlyContinue
  }
  if ($answer -ne 'ok') { throw "Windows refused the location policy: $answer" }
}

if ($PolicyResult) {
  try {
    Set-LocationPolicy
    Set-Content -Path $PolicyResult -Value 'ok'
  } catch {
    Set-Content -Path $PolicyResult -Value $_.Exception.Message
  }
  exit 0
}

if ($Run) {
  $ErrorActionPreference = 'Stop'
  Enable-Location
  $accounts = Enable-UserLocation
  Invoke-LocationPolicy
  Write-Host '  Location forced on for every account by device policy'
  Write-Host "  Location allowed for every account, $accounts of them signed out"
  $value = (Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\CapabilityAccessManager\ConsentStore\location').Value
  Write-Host "  Location for this device: $value"
}
