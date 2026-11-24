#Requires -Version 7.2
param([string]$Version='v0.1.0-pilot.3')
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'release-ops.ps1')
Release-LocalEngine
$operationLock=Release-Lock
try {
    $statePath=Join-Path $script:ReleaseRoot 'private/deployment.json'
    if (Test-Path $statePath) { throw 'Already installed. Use scripts/update.ps1 -Version <explicit-tag>.' }
    & (Join-Path $PSScriptRoot 'setup.ps1')
    $existing=Release-Docker @('compose','ps','--all','--quiet')
    $pendingPath=Join-Path $script:ReleaseRoot 'private/install-pending.json'
    if ($existing -and -not (Test-Path $pendingPath)) { throw 'Existing containers found. Use update.ps1 -Adopt for a reviewed, binary-identical source installation.' }
    if(-not $existing){
        $config=(Release-Docker @('compose','config','--format','json'))|ConvertFrom-Json
        $ports=@($config.services.api.ports)
        if($ports.Count -ne 1 -or $ports[0].host_ip -ne '127.0.0.1' -or $ports[0].target -ne 7474 -or $ports[0].protocol -ne 'tcp' -or $ports[0].published -notmatch '^\d{1,5}$'){
            throw 'Ready-image installation requires one loopback TCP API port; inspect Compose overrides.'
        }
        Release-RequireFreePort ([int]$ports[0].published)
    }
    $manifest=Release-ReadManifest $Version
    $imageID=Release-VerifyImage $manifest -Pull
    if(Test-Path $pendingPath){
        $pending=Get-Content $pendingPath -Raw|ConvertFrom-Json
        if($pending.image -ne $manifest.image -or $pending.image_id -ne $imageID){throw 'Retry the original install version; partial installation state was retained.'}
    }
    if($existing){
        $db=Release-Service 'db'
        if(-not $db.running){throw 'Start the existing metadata database and inspect its health before retrying installation.'}
        foreach($service in @('api','worker')){
            if(Release-Docker @('compose','ps','--all','--quiet',$service)){
                if((Release-Service $service).image -ne $imageID){throw 'Partial installation image drift detected.'}
            }
        }
    }else{Release-Docker @('pull','postgres:17-bookworm') 600 | Out-Null}
    Release-JSON $pendingPath @{image=$manifest.image;image_id=$imageID}
    $env:RESTIVRA_IMAGE=$manifest.image
    try { if(-not $existing){Release-Docker @('compose','up','--no-build','--pull','never','-d','--wait','--wait-timeout','120','db') 150 | Out-Null} }
    finally { Remove-Item Env:RESTIVRA_IMAGE -ErrorAction SilentlyContinue }
    Release-Start $manifest.image
    Release-SetImage $manifest.image
    $installation=Release-SQL 'SELECT id FROM installation'
    Release-JSON $statePath @{format='restivra.deployment.v1';installation=$installation;current=@{manifest=$manifest;image=$manifest.image;image_id=$imageID};previous=$null;checkpoint=$null}
    $config=(Release-Docker @('compose','config','--format','json'))|ConvertFrom-Json
    $port=$config.services.api.ports[0].published
    Write-Host "Installed $Version. Open http://127.0.0.1:$port. Sign in with the token in your private .env."
} finally { $operationLock.Dispose() }
