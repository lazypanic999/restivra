#Requires -Version 7.2
# Private files are owner-only before content is written. Windows inherits NTFS ACLs.
. (Join-Path $PSScriptRoot 'native-command.ps1')
function Get-RestivraUnixMode([string]$Path) {
    if([IO.FileStreamOptions].GetProperty('UnixCreateMode')){return [int][IO.File]::GetUnixFileMode($Path)}
    # PowerShell 7.2 uses .NET 6, before managed UnixFileMode APIs existed.
    $statArguments=if($IsMacOS){@('-f','%Lp',$Path)}else{@('-c','%a','--',$Path)}
    $r=Invoke-RestivraNative -FilePath stat -Arguments $statArguments -TimeoutSeconds 10
    if($r.ExitCode -ne 0 -or $r.Output.Trim() -notmatch '^[0-7]{3,4}$'){throw 'Cannot verify private Unix file permissions.'}
    return [Convert]::ToInt32($r.Output.Trim(),8)
}
function New-RestivraPrivateDirectory([string]$Path) {
    $Path=$ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($Path)
    if([IO.Directory]::Exists($Path)){return}
    if($IsWindows){$null=[IO.Directory]::CreateDirectory($Path);return}
    if([IO.FileStreamOptions].GetProperty('UnixCreateMode')){
        $null=[IO.Directory]::CreateDirectory($Path,[Enum]::ToObject([type]'System.IO.UnixFileMode',448))
    }else{
        $r=Invoke-RestivraNative -FilePath sh -Arguments @('-c','umask 077; mkdir -p -m 700 "$1"','restivra-private-directory',$Path) -TimeoutSeconds 10
        if($r.ExitCode -ne 0){throw 'Cannot create a private Unix directory.'}
    }
    if((Get-RestivraUnixMode $Path) -ne 448){throw 'Private directory permissions could not be enforced; use a filesystem supporting Unix permissions.'}
}
function New-RestivraPrivateFile([string]$Path) {
    $Path=$ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($Path)
    $stream=$null
    try{
        if($IsWindows){$stream=[IO.File]::Open($Path,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)}
        elseif([IO.FileStreamOptions].GetProperty('UnixCreateMode')){
            $options=[IO.FileStreamOptions]::new()
            $options.Mode=[IO.FileMode]::CreateNew;$options.Access=[IO.FileAccess]::Write;$options.Share=[IO.FileShare]::None
            $options.UnixCreateMode=[Enum]::ToObject([type]'System.IO.UnixFileMode',384)
            $stream=[IO.FileStream]::new($Path,$options)
        }else{
            # No secrets in argv or a public temporary file. POSIX noclobber
            # creates an empty 0600 file before the parent opens its stream.
            $r=Invoke-RestivraNative -FilePath sh -Arguments @('-c','umask 077; set -C; : > "$1"','restivra-private-file',$Path) -TimeoutSeconds 10
            if($r.ExitCode -ne 0){throw 'Cannot exclusively create a private Unix file.'}
            $stream=[IO.File]::Open($Path,[IO.FileMode]::Open,[IO.FileAccess]::Write,[IO.FileShare]::None)
        }
        if(-not $IsWindows -and (Get-RestivraUnixMode $Path) -ne 384){throw 'Private file permissions could not be enforced; use a filesystem supporting Unix permissions.'}
        return $stream
    }catch{if($stream){$stream.Dispose()};throw}
}
function Set-RestivraPrivateMode([string]$Path,[ValidateSet(384,448)][int]$Mode) {
    if($IsWindows){return}
    $Path=$ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($Path)
    if(([IO.File]::GetAttributes($Path) -band [IO.FileAttributes]::ReparsePoint) -ne 0){throw 'Review symbolic-link configuration permissions manually; no linked target was changed.'}
    if([IO.FileStreamOptions].GetProperty('UnixCreateMode')){
        [IO.File]::SetUnixFileMode($Path,[Enum]::ToObject([type]'System.IO.UnixFileMode',$Mode))
    }else{
        $r=Invoke-RestivraNative -FilePath chmod -Arguments @([Convert]::ToString($Mode,8),$Path) -TimeoutSeconds 10
        if($r.ExitCode -ne 0){throw 'Cannot restrict private Unix permissions.'}
    }
    if((Get-RestivraUnixMode $Path) -ne $Mode){throw 'Private Unix permissions could not be enforced.'}
}
