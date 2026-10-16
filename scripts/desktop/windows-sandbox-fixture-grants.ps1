# INPUT: check-windows-sandbox-logon 已核验 owner、账号与 GUID 的测试 fixture。
# OUTPUT: 仅该 fixture 的 AppContainer DACL 授权及可核对的恢复记录。
# POS: 测试准备，不改 owner/SACL、系统设备或卷根；不能作为生产 ACL 租约实现。
if (-not ('NexusSandboxTestDacl' -as [type])) {
    Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class NexusSandboxTestDacl {
 [DllImport("userenv.dll", CharSet=CharSet.Unicode)] public static extern int DeriveAppContainerSidFromAppContainerName(string name, out IntPtr sid);
 [DllImport("advapi32.dll")] public static extern IntPtr FreeSid(IntPtr sid);
 [DllImport("advapi32.dll")] public static extern bool GetSecurityDescriptorDacl(IntPtr sd, out bool present, out IntPtr dacl, out bool defaulted);
 [DllImport("advapi32.dll", CharSet=CharSet.Unicode)] public static extern uint SetNamedSecurityInfoW(string path, uint kind, uint info, IntPtr owner, IntPtr group, IntPtr dacl, IntPtr sacl);
}
'@
}
function Get-NexusFixtureDacl([string]$Path) {
    if (((Get-Item -LiteralPath $Path -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) { throw 'Fixture grant became a reparse point.' }
    $acl = Get-Acl -LiteralPath $Path
    if (-not $acl.AreAccessRulesProtected) { throw 'Fixture grant requires the prepared protected DACL.' }
    return $acl.GetSecurityDescriptorSddlForm([Security.AccessControl.AccessControlSections]::Access)
}
function Set-NexusFixtureDacl([string]$Path, [string]$Sddl) {
    $null = Get-NexusFixtureDacl $Path
    $acl = [Security.AccessControl.DirectorySecurity]::new()
    $acl.SetSecurityDescriptorSddlForm($Sddl, [Security.AccessControl.AccessControlSections]::Access)
    $bytes = $acl.GetSecurityDescriptorBinaryForm()
    $sd = [Runtime.InteropServices.Marshal]::AllocHGlobal($bytes.Length)
    try {
        [Runtime.InteropServices.Marshal]::Copy($bytes,0,$sd,$bytes.Length)
        $present=$false; $defaulted=$false; $dacl=[IntPtr]::Zero
        if (-not [NexusSandboxTestDacl]::GetSecurityDescriptorDacl($sd,[ref]$present,[ref]$dacl,[ref]$defaulted) -or -not $present -or $dacl -eq [IntPtr]::Zero) { throw 'Missing fixture DACL.' }
        $result = [NexusSandboxTestDacl]::SetNamedSecurityInfoW($Path,1,[uint32]2147483652,[IntPtr]::Zero,[IntPtr]::Zero,$dacl,[IntPtr]::Zero)
        if ($result -ne 0) { throw "Fixture DACL update failed: $result" }
    } finally { [Runtime.InteropServices.Marshal]::FreeHGlobal($sd) }
}
function Remove-NexusFixtureAppContainerGrants([object[]]$Grants, [string]$ReportDirectory) {
    $errors = @()
    foreach ($grant in $Grants) {
        try {
            $current = Get-NexusFixtureDacl $grant.path
            if ($current -ne $grant.before) {
                if ($current -ne $grant.applied -and $current -ne $grant.requested) { throw 'Fixture DACL changed concurrently; recorded original requires explicit reconciliation.' }
                Set-NexusFixtureDacl $grant.path $grant.before
                if ((Get-NexusFixtureDacl $grant.path) -ne $grant.before) { throw 'Fixture DACL restoration did not match.' }
            }
            $grant.state = 'restored'
        } catch { $errors += "$($grant.path): $($_.Exception.Message)" }
    }
    $Grants | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $ReportDirectory 'fixture-grants.json')
    @{restored=($errors.Count -eq 0); errors=$errors} | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $ReportDirectory 'fixture-cleanup.json')
    if ($errors.Count -ne 0) { throw ($errors -join "`n") }
}
function New-NexusFixtureAppContainerGrants([string]$FixtureRoot, [string]$ReportDirectory) {
    $fixtureID = Split-Path -Leaf $FixtureRoot
    $parent = [IO.Path]::GetFullPath((Join-Path $env:ProgramData 'NexusSandboxTests'))
    if ((Split-Path -Parent ([IO.Path]::GetFullPath($FixtureRoot))) -ine $parent -or $fixtureID -notmatch '^[a-f0-9]{32}$') { throw 'Unexpected AppContainer fixture root.' }
    $pointer = [IntPtr]::Zero
    $result = [NexusSandboxTestDacl]::DeriveAppContainerSidFromAppContainerName(('nexus-p3-'+$fixtureID),[ref]$pointer)
    if ($result -ne 0) { throw "Cannot derive fixture AppContainer SID: $result" }
    try { $sid = [Security.Principal.SecurityIdentifier]::new($pointer) }
    finally { [NexusSandboxTestDacl]::FreeSid($pointer) | Out-Null }
    $grants = @()
    try {
        foreach ($relative in @('','bin','temp')) {
            $path = if ($relative) {Join-Path $FixtureRoot $relative} else {$FixtureRoot}
            $before = Get-NexusFixtureDacl $path
            $acl = [Security.AccessControl.DirectorySecurity]::new()
            $acl.SetSecurityDescriptorSddlForm($before,[Security.AccessControl.AccessControlSections]::Access)
            if (@($acl.GetAccessRules($true,$false,[Security.Principal.SecurityIdentifier]) | Where-Object {$_.IdentityReference -eq $sid}).Count -ne 0) { throw 'Existing fixture grants require explicit reconciliation.' }
            $rights = if ($relative -eq 'temp') {'Modify'} else {'ReadAndExecute'}
            $rule = [Security.AccessControl.FileSystemAccessRule]::new($sid,[Security.AccessControl.FileSystemRights]$rights,[Security.AccessControl.InheritanceFlags]'ContainerInherit,ObjectInherit',[Security.AccessControl.PropagationFlags]::None,[Security.AccessControl.AccessControlType]::Allow)
            $acl.AddAccessRule($rule)
            $grant = @{path=$path; before=$before; requested=$acl.GetSecurityDescriptorSddlForm([Security.AccessControl.AccessControlSections]::Access); applied=$null; state='prepared'}
            $grants += $grant
            $grants | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $ReportDirectory 'fixture-grants.json')
            Set-NexusFixtureDacl $path $grant.requested
            $grant.applied = Get-NexusFixtureDacl $path
            $grant.state = 'applied'
            $grants | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $ReportDirectory 'fixture-grants.json')
        }
        return $grants
    } catch {
        Remove-NexusFixtureAppContainerGrants $grants $ReportDirectory
        throw
    }
}
