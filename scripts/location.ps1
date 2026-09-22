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
