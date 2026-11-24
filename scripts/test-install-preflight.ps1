#Requires -Version 7.2
# A held local port must fail before manifest access or Docker data creation.
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'release-ops.ps1')
Release-LocalEngine
$sourceRoot=$script:ReleaseRoot
$suffix=[guid]::NewGuid().ToString('N')
$root=Join-Path $sourceRoot "work/install-preflight-$suffix"
$null=New-Item -ItemType Directory $root
Copy-Item (Join-Path $sourceRoot 'compose.yaml') $root
Copy-Item (Join-Path $sourceRoot 'scripts') (Join-Path $root 'scripts') -Recurse
$oldProject=$env:COMPOSE_PROJECT_NAME;$oldPort=$env:RESTIVRA_PORT
$project="restivra-preflight-$($suffix.Substring(0,12))"
$listener=[Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,0)
$listener.ExclusiveAddressUse=$true
$listener.Start()
$env:COMPOSE_PROJECT_NAME=$project;$env:RESTIVRA_PORT="$($listener.LocalEndpoint.Port)"
$script:ReleaseRoot=$root
try{
    & (Join-Path $root 'scripts/setup.ps1')
    $settings=(Get-FileHash (Join-Path $root '.env')).Hash
    $profiles=(Get-FileHash (Join-Path $root 'private/sources.json')).Hash
    $refused=$false
    # The deliberately invalid version would fail differently if the installer
    # tried to reach a registry before rejecting the local conflict.
    try{& (Join-Path $root 'scripts/install.ps1') -Version 'invalid-preflight-test'}
    catch{if($_.Exception.Message -match 'API port is unavailable'){$refused=$true}else{throw}}
    if(-not $refused){throw 'Fresh installation did not reject the occupied port.'}
    if(Release-Docker @('ps','--all','--quiet','--filter',"label=com.docker.compose.project=$project")){throw 'Port rejection created a container.'}
    if(Release-Docker @('volume','ls','--quiet','--filter',"label=com.docker.compose.project=$project")){throw 'Port rejection created a data volume.'}
    if((Get-FileHash (Join-Path $root '.env')).Hash -cne $settings -or (Get-FileHash (Join-Path $root 'private/sources.json')).Hash -cne $profiles){throw 'Port rejection replaced private settings.'}
    if(Test-Path (Join-Path $root 'private/install-pending.json')){throw 'Port rejection recorded a partial installation.'}
    Write-Host 'Occupied API port refused before manifest access, container/volume creation or private-settings changes.'
}finally{
    $listener.Stop()
    $script:ReleaseRoot=$sourceRoot;$env:COMPOSE_PROJECT_NAME=$oldProject;$env:RESTIVRA_PORT=$oldPort
}
