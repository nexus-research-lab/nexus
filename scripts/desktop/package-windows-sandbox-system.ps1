# INPUT: independently built signed SYSTEM service, SDK source, signing certificate and WiX CLI.
# OUTPUT: a separate machine MSI with an embedded SYSTEM preflight executable bound to the service SHA256.
# POS: package construction only; never installs, elevates, starts a service or changes machine ACLs.
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$ServiceBinary,
    [Parameter(Mandatory = $true)][string]$BootstrapBinary,
    [Parameter(Mandatory = $true)][string]$SDKDirectory,
    [Parameter(Mandatory = $true)][string]$PackageVersion,
    [Parameter(Mandatory = $true)][string]$OutputDirectory,
    [Parameter(Mandatory = $true)][string]$SigningCertificateThumbprint,
    [Parameter(Mandatory = $true)][string]$SignToolPath,
    [ValidateSet('x64', 'arm64')][string]$Architecture = 'x64',
    [string]$WixPath = 'wix',
    [string]$TimestampURL = 'http://timestamp.digicert.com'
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

# 读取实际 PE 头，不能仅依赖包名或构建参数推断机器映像的架构。
function Assert-MachineImageArchitecture {
    param([string]$ImagePath, [string]$ExpectedArchitecture)
    $stream = [System.IO.File]::Open($ImagePath, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, [System.IO.FileShare]::Read)
    $reader = $null
    try {
        $reader = [System.IO.BinaryReader]::new($stream)
        if ($stream.Length -lt 64 -or $reader.ReadUInt16() -ne 0x5a4d) { throw 'Machine image has no valid DOS header' }
        $stream.Position = 0x3c
        $headerOffset = $reader.ReadUInt32()
        if ($headerOffset -lt 64 -or [long]$headerOffset + 24 -gt $stream.Length) { throw 'Machine image PE header is outside the file' }
        $stream.Position = $headerOffset
        if ($reader.ReadUInt32() -ne 0x00004550) { throw 'Machine image has no valid PE signature' }
        $machine = $reader.ReadUInt16()
        $expectedMachine = if ($ExpectedArchitecture -eq 'x64') { 0x8664 } else { 0xaa64 }
        if ($machine -ne $expectedMachine) { throw "Machine image architecture differs from $ExpectedArchitecture package: $ImagePath" }
    } finally {
        if ($null -ne $reader) { $reader.Dispose() } else { $stream.Dispose() }
    }
}

# 签名工具零退出之外，还要检查实际产物和固定发布证书。
function Assert-ReleaseSignature {
    param([string]$ArtifactPath, [string]$CertificateThumbprint)
    $signature = Get-AuthenticodeSignature -LiteralPath $ArtifactPath
    if ($signature.Status -ne 'Valid' -or $null -eq $signature.SignerCertificate -or $signature.SignerCertificate.Thumbprint -ne $CertificateThumbprint) {
        throw "Artifact lacks the configured valid release signature: $ArtifactPath"
    }
}

if ($PackageVersion -notmatch '^\d+\.\d+\.\d+$') { throw 'PackageVersion must contain three numeric components' }
if ($SigningCertificateThumbprint -notmatch '^[a-fA-F0-9]{40}$') { throw 'A fixed signing certificate thumbprint is required' }
$servicePath = (Resolve-Path -LiteralPath $ServiceBinary).Path
$bootstrapPath = (Resolve-Path -LiteralPath $BootstrapBinary).Path
$sdkPath = (Resolve-Path -LiteralPath $SDKDirectory).Path
$signTool = (Resolve-Path -LiteralPath $SignToolPath).Path
Assert-MachineImageArchitecture $servicePath $Architecture
Assert-ReleaseSignature $servicePath $SigningCertificateThumbprint
Assert-MachineImageArchitecture $bootstrapPath $Architecture
Assert-ReleaseSignature $bootstrapPath $SigningCertificateThumbprint
$serviceDigest = (Get-FileHash -LiteralPath $servicePath -Algorithm SHA256).Hash.ToLowerInvariant()
$bootstrapDigest = (Get-FileHash -LiteralPath $bootstrapPath -Algorithm SHA256).Hash.ToLowerInvariant()
$outputPath = [System.IO.Path]::GetFullPath($OutputDirectory)
New-Item -ItemType Directory -Path $outputPath -Force | Out-Null
$setupPath = Join-Path $outputPath 'nxs-sandbox-system-setup.exe'
$msiPath = Join-Path $outputPath ("nexus-sandbox-system-$PackageVersion-$Architecture.msi")
$sourcePath = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../../desktop/windows/sandbox-system/Package.wxs'))
$savedWork = $env:GOWORK
$savedOS = $env:GOOS
$savedArch = $env:GOARCH
try {
    $env:GOWORK = 'off'
    $env:GOOS = 'windows'
    $env:GOARCH = if ($Architecture -eq 'x64') { 'amd64' } else { 'arm64' }
    Push-Location -LiteralPath $sdkPath
    try {
        & go build -trimpath -ldflags "-X main.serviceImageDigest=$serviceDigest -X main.bootstrapImageDigest=$bootstrapDigest" -o $setupPath './cmd/nxs-sandbox-system-setup'
        if ($LASTEXITCODE -ne 0) { throw 'Machine setup executable build failed' }
    } finally { Pop-Location }
} finally {
    $env:GOWORK = $savedWork
    $env:GOOS = $savedOS
    $env:GOARCH = $savedArch
}
& $signTool sign /sha1 $SigningCertificateThumbprint /fd SHA256 /tr $TimestampURL /td SHA256 $setupPath
if ($LASTEXITCODE -ne 0) { throw 'Machine setup executable signing failed' }
Assert-MachineImageArchitecture $setupPath $Architecture
Assert-ReleaseSignature $setupPath $SigningCertificateThumbprint
if ((Get-FileHash -LiteralPath $servicePath -Algorithm SHA256).Hash.ToLowerInvariant() -ne $serviceDigest) {
    throw 'Machine service changed while constructing its bound setup executable'
}
if ((Get-FileHash -LiteralPath $bootstrapPath -Algorithm SHA256).Hash.ToLowerInvariant() -ne $bootstrapDigest) {
    throw 'Machine bootstrap changed while constructing its bound setup executable'
}
& $WixPath build $sourcePath -arch $Architecture -d "ServiceBinary=$servicePath" -d "BootstrapBinary=$bootstrapPath" -d "SetupBinary=$setupPath" -d "PackageVersion=$PackageVersion" -o $msiPath
if ($LASTEXITCODE -ne 0) { throw 'Machine MSI build failed' }
& $signTool sign /sha1 $SigningCertificateThumbprint /fd SHA256 /tr $TimestampURL /td SHA256 $msiPath
if ($LASTEXITCODE -ne 0) { throw 'Machine MSI signing failed' }
Assert-ReleaseSignature $msiPath $SigningCertificateThumbprint
Write-Output $msiPath
