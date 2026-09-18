<#
  Builds an MSIX so Windows will offer the stand in
  Settings, Accounts, Set up a kiosk.

  That picker lists Microsoft Edge and installed packaged apps and nothing
  else, so a plain executable can never appear in it. Packaging is the only
  route, and a package has to be signed, so this makes a certificate, trusts
  it on this machine, and installs the result.

  Run scripts\package.bat, which elevates and calls this.
#>

param(
  [string]$Root = (Split-Path -Parent $PSScriptRoot),
  [string]$Version = "1.0.0.0"
)

$ErrorActionPreference = "Stop"
$build = Join-Path $Root "build\msix"
$msix  = Join-Path $Root "build\UpgamingRoaster.msix"

function Find-SdkTool([string]$name) {
  $tool = Get-ChildItem "${env:ProgramFiles(x86)}\Windows Kits\10\bin" -Recurse -Filter $name -ErrorAction SilentlyContinue |
          Where-Object { $_.FullName -match "\\x64\\" } |
          Sort-Object FullName -Descending | Select-Object -First 1
  if (-not $tool) {
    throw "$name is missing. Install the Windows SDK: winget install Microsoft.WindowsSDK"
  }
  return $tool.FullName
}

Write-Host "  Building..."
Push-Location $Root
try {
  & go build -o (Join-Path $Root "roaster.exe") ".\cmd\roaster"
  if ($LASTEXITCODE) { throw "roaster.exe failed to build" }
  & go build -ldflags="-s -w -H windowsgui" -o (Join-Path $Root "kiosk.exe") ".\cmd\kiosk"
  if ($LASTEXITCODE) { throw "kiosk.exe failed to build" }
} finally { Pop-Location }

Write-Host "  Laying out the package..."
Remove-Item $build -Recurse -Force -ErrorAction SilentlyContinue
New-Item $build -ItemType Directory -Force | Out-Null

Copy-Item (Join-Path $Root "packaging\AppxManifest.xml") $build
Copy-Item (Join-Path $Root "packaging\images") $build -Recurse
Copy-Item (Join-Path $Root "kiosk.exe")   $build
Copy-Item (Join-Path $Root "roaster.exe") $build
if (Test-Path (Join-Path $Root "roaster.json")) {
  Copy-Item (Join-Path $Root "roaster.json") $build
} else {
  Copy-Item (Join-Path $Root "roaster.example.json") (Join-Path $build "roaster.json")
}
if (Test-Path (Join-Path $Root "events")) { Copy-Item (Join-Path $Root "events") $build -Recurse }

# The manifest version has to move for Windows to accept a reinstall.
$manifestPath = Join-Path $build "AppxManifest.xml"
(Get-Content $manifestPath -Raw) -replace 'Version="1\.0\.0\.0"', "Version=""$Version""" |
  Set-Content $manifestPath -Encoding UTF8

Write-Host "  Packing..."
& (Find-SdkTool "makeappx.exe") pack /d $build /p $msix /o | Out-Null
if ($LASTEXITCODE) { throw "makeappx failed" }

# Publisher here must match the manifest exactly or the package will not install.
$subject = "CN=Upgaming"
$cert = Get-ChildItem Cert:\CurrentUser\My | Where-Object { $_.Subject -eq $subject } | Select-Object -First 1
if (-not $cert) {
  Write-Host "  Making a signing certificate..."
  $cert = New-SelfSignedCertificate -Type Custom -Subject $subject `
    -KeyUsage DigitalSignature -FriendlyName "Upgaming Roaster kiosk" `
    -CertStoreLocation "Cert:\CurrentUser\My" `
    -TextExtension @("2.5.29.37={text}1.3.6.1.5.5.7.3.3", "2.5.29.19={text}")
}

Write-Host "  Trusting it on this machine..."
$cerPath = Join-Path $Root "build\Upgaming.cer"
Export-Certificate -Cert $cert -FilePath $cerPath | Out-Null
Import-Certificate -FilePath $cerPath -CertStoreLocation "Cert:\LocalMachine\TrustedPeople" | Out-Null

Write-Host "  Signing..."
& (Find-SdkTool "signtool.exe") sign /fd SHA256 /sha1 $cert.Thumbprint $msix | Out-Null
if ($LASTEXITCODE) { throw "signtool failed" }

Write-Host "  Installing..."
Add-AppxPackage -Path $msix -ForceUpdateFromAnyVersion

Write-Host ""
Write-Host "  Installed as Upgaming Roaster."
Write-Host "  Settings, Accounts, Other users, Set up a kiosk, and it is in the list."
Write-Host ""
