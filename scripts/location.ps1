param([switch]$Run)

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

if ($Run) {
  $ErrorActionPreference = 'Stop'
  Enable-Location
  $accounts = Enable-UserLocation
  Write-Host "  Location allowed for every account, $accounts of them signed out"
  $value = (Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\CapabilityAccessManager\ConsentStore\location').Value
  Write-Host "  Location for this device: $value"
}
