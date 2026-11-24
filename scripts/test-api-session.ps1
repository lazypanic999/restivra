#Requires -Version 7.2
# Origin selection is tested without sending credentials or making HTTP calls.
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'api-session.ps1')
$root=Split-Path -Parent $PSScriptRoot
$scratch=Join-Path $root ('work/api-session-'+[guid]::NewGuid().ToString('N'))
$null=New-Item -ItemType Directory $scratch
$file=Join-Path $scratch '.env'
$oldPort=$env:RESTIVRA_PORT
function Require-Base([string]$Expected,[string]$Base=''){
    if((Resolve-RestivraBase -Base $Base -EnvFile $file) -cne $Expected){throw 'API origin did not match the selected installation.'}
}
function Refuse-Base([string]$Base=''){
    $refused=$false
    try{$null=Resolve-RestivraBase -Base $Base -EnvFile $file}catch{$refused=$true}
    if(-not $refused){throw 'Unsafe or malformed API origin was accepted.'}
}
try{
    $env:RESTIVRA_PORT=$null
    Require-Base 'http://127.0.0.1:7474'
    [IO.File]::WriteAllText($file,"RESTIVRA_PORT=27486`n")
    Require-Base 'http://127.0.0.1:27486'
    $env:RESTIVRA_PORT='27487'
    Require-Base 'http://127.0.0.1:27487'
    Require-Base 'http://localhost:27488' 'http://localhost:27488/'
    Require-Base 'https://example.com' 'https://example.com/'
    $env:RESTIVRA_PORT=$null
    [IO.File]::WriteAllText($file,"RESTIVRA_PORT=27486`nRESTIVRA_PORT=27487`n")
    Refuse-Base
    foreach($bad in @('80','65536','abc',('7474'+"`nOTHER=value"))){
        [IO.File]::WriteAllText($file,"RESTIVRA_PORT=$bad`n")
        if($bad -like '7474*'){
            # A separate .env setting is never interpreted as part of the port.
            Require-Base 'http://127.0.0.1:7474'
        }else{Refuse-Base}
    }
    foreach($bad in @('http://example.com','ftp://localhost','http://localhost:7474/path','http://localhost:7474/?token=x','https://user:pass@example.com','http://localhost:7474/#secret','not-an-origin')){Refuse-Base $bad}
    Write-Host 'API origin checks passed: saved port, terminal precedence, explicit origin, legacy default and malformed/embedded-credential URL rejection.'
}finally{$env:RESTIVRA_PORT=$oldPort}
