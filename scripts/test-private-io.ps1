#Requires -Version 7.2
param([string]$ScratchRoot=(Join-Path (Split-Path -Parent $PSScriptRoot) 'work'))
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'private-io.ps1')
$root=Join-Path $ScratchRoot ('private-io-'+[guid]::NewGuid().ToString('N'))
New-RestivraPrivateDirectory $root
$path=Join-Path $root 'literal $() [spaces] Türkçe.json'
$content=[Text.Encoding]::UTF8.GetBytes('SYNTHETIC_PRIVATE_IO_CANARY')
$stream=New-RestivraPrivateFile $path
try{
    if(-not $IsWindows -and (Get-RestivraUnixMode $path) -ne 384){throw 'Private file was public before its content was written.'}
    $stream.Write($content,0,$content.Length);$stream.Flush($true)
}finally{$stream.Dispose()}
if([IO.File]::ReadAllText($path) -cne 'SYNTHETIC_PRIVATE_IO_CANARY'){throw 'Private file content or literal path was changed.'}
$refused=$false
try{$other=New-RestivraPrivateFile $path;$other.Dispose()}catch{$refused=$true}
if(-not $refused -or [IO.File]::ReadAllText($path) -cne 'SYNTHETIC_PRIVATE_IO_CANARY'){throw 'Private creation overwrote an existing file.'}
if(-not $IsWindows){
    if((Get-RestivraUnixMode $root) -ne 448){throw 'Private directory is not owner-only.'}
    $r=Invoke-RestivraNative -FilePath chmod -Arguments @('644',$path)
    if($r.ExitCode -ne 0){throw 'Cannot prepare synthetic legacy permissions.'}
    $link=Join-Path $root 'linked.json'
    $null=[IO.File]::CreateSymbolicLink($link,$path)
    $refused=$false
    try{Set-RestivraPrivateMode $link 384}catch{$refused=$true}
    if(-not $refused -or (Get-RestivraUnixMode $path) -ne 420){throw 'Private permission update followed a symbolic link.'}
    Set-RestivraPrivateMode $path 384
    if([IO.File]::ReadAllText($path) -cne 'SYNTHETIC_PRIVATE_IO_CANARY'){throw 'Permission hardening changed existing content.'}
}
Write-Host "Private I/O passed on PowerShell $($PSVersionTable.PSVersion): exclusive creation, literal paths, content preservation and platform permissions."
