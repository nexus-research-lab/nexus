# INPUT: 原 owner SID、独立 fixture 路径与已构建测试二进制的 SHA-256。
# OUTPUT: 可显式移除的普通测试账号、最小 fixture ACL 与 owner DPAPI 凭据。
# POS: 本机 Windows P3 测试设施；不是产品安装器，不接受模型输入。
[CmdletBinding()]
param(
    [Parameter(Mandatory)][ValidateSet('Create', 'Remove')][string]$Action,
    [Parameter(Mandatory)][string]$FixtureRoot,
    [Parameter(Mandatory)][string]$OwnerSid,
    [string]$BinaryPath,
    [string]$BinarySHA256
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = [Security.Principal.WindowsPrincipal]::new($identity)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw 'Fixture account provisioning requires an elevated Windows administrator process.'
}
if ($identity.User.Value -ne $OwnerSid) {
    throw 'This test helper requires elevation of the original owner; alternate administrator identities are not supported.'
}

$fixtureBase = [IO.Path]::GetFullPath((Join-Path $env:ProgramData 'NexusSandboxTests'))
$FixtureRoot = [IO.Path]::GetFullPath($FixtureRoot).TrimEnd('\')
$fixtureID = Split-Path -Leaf $FixtureRoot
if ((Split-Path -Parent $FixtureRoot) -ine $fixtureBase -or $fixtureID -notmatch '^[a-f0-9]{32}$') {
    throw 'FixtureRoot must be one GUID-named direct child of ProgramData\NexusSandboxTests.'
}
# 提升权限前后的输入都可能指向链接；拒绝既有祖先的任何 reparse 跳转。
$ancestor = $FixtureRoot
while ($ancestor) {
    if (Test-Path -LiteralPath $ancestor) {
        if (((Get-Item -LiteralPath $ancestor -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
            throw "Fixture ancestor is a reparse point: $ancestor"
        }
    }
    $ancestor = Split-Path -Parent $ancestor
}

$controlRoot = Join-Path $FixtureRoot 'control'
$manifestPath = Join-Path $controlRoot 'fixture.json'
$expectedDescription = "Nexus test $fixtureID"

# Set-FixtureACL 不修改系统或用户目录，只设置本 fixture 的受保护 DACL。
function Set-FixtureACL([string]$Path, [hashtable]$Grants) {
    $acl = [Security.AccessControl.DirectorySecurity]::new()
    $acl.SetAccessRuleProtection($true, $false)
    $acl.SetOwner([Security.Principal.SecurityIdentifier]::new($OwnerSid))
    foreach ($entry in $Grants.GetEnumerator()) {
        $rule = [Security.AccessControl.FileSystemAccessRule]::new(
            [Security.Principal.SecurityIdentifier]::new($entry.Key),
            [Security.AccessControl.FileSystemRights]$entry.Value,
            [Security.AccessControl.InheritanceFlags]'ContainerInherit,ObjectInherit',
            [Security.AccessControl.PropagationFlags]::None,
            [Security.AccessControl.AccessControlType]::Allow)
        $acl.AddAccessRule($rule)
    }
    Set-Acl -LiteralPath $Path -AclObject $acl
}

if ($Action -eq 'Remove') {
    $manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
    if ($manifest.version -ne 1 -or $manifest.fixtureID -ne $fixtureID -or $manifest.ownerSid -ne $OwnerSid) {
        throw 'Fixture ownership metadata does not match; refusing removal.'
    }
    $account = Get-LocalUser -Name $manifest.accountName -ErrorAction SilentlyContinue
    if ($account) {
        if ($account.SID.Value -ne $manifest.accountSid -or $account.Description -ne $expectedDescription) {
            throw 'Account identity changed; refusing removal.'
        }
        # 已创建进程不会因账号删除而结束；必须先证明该测试身份没有活动进程。
        foreach ($process in Get-CimInstance Win32_Process) {
            $owner = Invoke-CimMethod -InputObject $process -MethodName GetOwnerSid -ErrorAction SilentlyContinue
            if ($owner -and $owner.ReturnValue -eq 0 -and $owner.Sid -eq $account.SID.Value) {
                throw "Fixture process $($process.ProcessId) is still active; close its Job before removal."
            }
        }
        # Start-Process -LoadUserProfile 的对照会物化 profile；按已核验 SID 删除，
        # 不能仅删除 SAM 账号后留下用户目录、注册表 hive 与 DPAPI 状态。
        $profiles = @(Get-CimInstance Win32_UserProfile | Where-Object { $_.SID -eq $account.SID.Value })
        if ($profiles.Count -gt 1) { throw 'Ambiguous fixture user profile.' }
        foreach ($profile in $profiles) {
            if ($profile.Loaded -or $profile.Special) { throw 'Fixture user profile is loaded or special; refusing removal.' }
            $profilesDirectory = [Environment]::ExpandEnvironmentVariables((Get-ItemProperty -LiteralPath 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\ProfileList').ProfilesDirectory)
            $profilePath = [IO.Path]::GetFullPath($profile.LocalPath).TrimEnd('\')
            if ((Split-Path -Parent $profilePath) -ine [IO.Path]::GetFullPath($profilesDirectory).TrimEnd('\') -or
                (Split-Path -Leaf $profilePath) -notmatch ('^' + [regex]::Escape($account.Name) + '(\.[A-Za-z0-9_-]+)*$')) {
                throw 'Fixture profile escaped its expected account directory.'
            }
            if ((Test-Path -LiteralPath $profilePath) -and
                (((Get-Item -LiteralPath $profilePath -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0)) {
                throw 'Fixture profile root is a reparse point.'
            }
            # Windows 自身删除已卸载 profile，避免手工递归其兼容 junction。
            Remove-CimInstance -InputObject $profile
        }
        Remove-LocalUser -SID $account.SID
    }
    # 不递归进入 junction；先移除 fixture 内的链接入口，再删除已核验的精确根。
    $directories = [Collections.Generic.Stack[string]]::new()
    $directories.Push($FixtureRoot)
    while ($directories.Count -gt 0) {
        foreach ($entry in Get-ChildItem -LiteralPath $directories.Pop() -Force) {
            if (($entry.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
                if ($entry.PSIsContainer) { [IO.Directory]::Delete($entry.FullName) }
                else { [IO.File]::Delete($entry.FullName) }
            } elseif ($entry.PSIsContainer) { $directories.Push($entry.FullName) }
        }
    }
    $resolvedFixture = [IO.Path]::GetFullPath($FixtureRoot)
    if ((Split-Path -Parent $resolvedFixture) -ine $fixtureBase) { throw 'Fixture escaped its base.' }
    Remove-Item -LiteralPath $resolvedFixture -Recurse -Force
    Write-Output "Removed Nexus test fixture $fixtureID"
    exit 0
}

if (Test-Path -LiteralPath $FixtureRoot) { throw 'Fixture already exists; use a new GUID.' }
if (-not $BinaryPath -or $BinarySHA256 -notmatch '^[a-fA-F0-9]{64}$') { throw 'An exact test binary and SHA-256 are required.' }
$binary = Get-Item -LiteralPath $BinaryPath
if ($binary.PSIsContainer -or (Get-FileHash -LiteralPath $binary.FullName -Algorithm SHA256).Hash -ine $BinarySHA256) {
    throw 'Test binary identity does not match the requested SHA-256.'
}

$accountName = 'nxs-test-' + $fixtureID.Substring(0, 8)
if (Get-LocalUser -Name $accountName -ErrorAction SilentlyContinue) { throw 'Test account name already exists.' }
$account = $null
$passwordText = 'Aa1!' + [Guid]::NewGuid().ToString('N') + [Guid]::NewGuid().ToString('N')
try {
    $securePassword = ConvertTo-SecureString $passwordText -AsPlainText -Force
    $account = New-LocalUser -Name $accountName -Password $securePassword -Description $expectedDescription -UserMayNotChangePassword -AccountExpires ([DateTime]::Now.AddDays(1))
    $users = Get-LocalGroup -SID 'S-1-5-32-545'
    if (-not (Get-LocalGroupMember -SID $users.SID | Where-Object { $_.SID -eq $account.SID })) {
        Add-LocalGroupMember -SID $users.SID -Member $account
    }
    $accountSid = $account.SID.Value
    New-Item -ItemType Directory -Path $FixtureRoot -Force | Out-Null
    $ownerGrants = @{$OwnerSid = 'FullControl'; 'S-1-5-18' = 'FullControl'; 'S-1-5-32-544' = 'FullControl'}
    $rootGrants = $ownerGrants.Clone()
    $rootGrants[$accountSid] = 'ReadAndExecute'
    Set-FixtureACL $FixtureRoot $rootGrants
    foreach ($name in @('control', 'bin', 'logs', 'temp')) {
        New-Item -ItemType Directory -Path (Join-Path $FixtureRoot $name) | Out-Null
    }
    Set-FixtureACL $controlRoot $ownerGrants
    $capabilitySid = 'S-1-5-21-161991819-20388491-918117717-306'
    $binaryGrants = $rootGrants.Clone()
    $binaryGrants[$capabilitySid] = 'ReadAndExecute'
    Set-FixtureACL (Join-Path $FixtureRoot 'bin') $binaryGrants
    $outputGrants = $ownerGrants.Clone()
    $outputGrants[$accountSid] = 'Modify'
    Set-FixtureACL (Join-Path $FixtureRoot 'logs') $outputGrants
    $outputGrants[$capabilitySid] = 'Modify'
    Set-FixtureACL (Join-Path $FixtureRoot 'temp') $outputGrants
    $destination = Join-Path $FixtureRoot 'bin\sandbox-components.test.exe'
    Copy-Item -LiteralPath $binary.FullName -Destination $destination
    if ((Get-FileHash -LiteralPath $destination -Algorithm SHA256).Hash -ine $BinarySHA256) { throw 'Copied test binary changed.' }
    # CLIXML 只保存原 owner 的 DPAPI 密文；命令参数、日志和任务环境不承载密码。
    [PSCredential]::new(".\$accountName", $securePassword) | Export-Clixml -LiteralPath (Join-Path $controlRoot 'credential.xml')
    @{
        version = 1; fixtureID = $fixtureID; ownerSid = $OwnerSid; accountName = $accountName
        accountSid = $accountSid; binarySHA256 = $BinarySHA256.ToLowerInvariant(); createdAt = [DateTime]::UtcNow.ToString('o')
    } | ConvertTo-Json | Set-Content -LiteralPath $manifestPath -Encoding utf8
    Write-Output "Created Nexus test fixture $fixtureID with ordinary account $accountName"
} catch {
    # 创建失败也只删除本次新建账号；保留目录供核对，不做未核验的递归删除。
    if ($account) { Remove-LocalUser -SID $account.SID -ErrorAction SilentlyContinue }
    throw
} finally {
    $passwordText = $null
    $securePassword = $null
}
