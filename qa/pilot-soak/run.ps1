#Requires -Version 7.2
# Self-contained real restore/fault soak; creates a new synthetic installation.
param(
    [Parameter(Mandatory)][string]$ApplicationImage,
    [ValidateRange(3,180)][int]$Minutes=120,
    [ValidateRange(1024,65535)][int]$Port=27484
)
$ErrorActionPreference='Stop'
$sourceRoot=Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
. (Join-Path $sourceRoot 'scripts/release-ops.ps1')
Release-LocalEngine
$imageID=Release-ImageID $ApplicationImage
$suffix=[guid]::NewGuid().ToString('N')
$testRoot=Join-Path $sourceRoot "work/soak-$suffix"
$null=New-Item -ItemType Directory $testRoot
Copy-Item (Join-Path $sourceRoot 'compose.yaml') $testRoot
Copy-Item (Join-Path $sourceRoot 'scripts') (Join-Path $testRoot 'scripts') -Recurse
$source="restivra-soak-source-$suffix"
$sourceCreated=$false
$oldProject=$env:COMPOSE_PROJECT_NAME;$oldPort=$env:RESTIVRA_PORT
$project="restivra-pilot-soak-$($suffix.Substring(0,12))"
$env:COMPOSE_PROJECT_NAME=$project;$env:RESTIVRA_PORT="$Port"
$script:ReleaseRoot=$testRoot
$proof=[ordered]@{status='failed';synthetic_data=$true;image=$imageID;requested_minutes=$Minutes;started_at=[datetime]::UtcNow.ToString('o')}
try {
    & (Join-Path $testRoot 'scripts/setup.ps1')
    Release-SetImage $imageID
    # No ports or external network for the disposable source database.
    Release-Docker @('run','--detach','--name',$source,'--network','none','--memory','768m','--cpus','1','--pids-limit','128','--label','restivra.pilot-fixture=true','--env','POSTGRES_HOST_AUTH_METHOD=trust','--env','POSTGRES_DB=commerce','postgres:17-bookworm')|Out-Null
    $sourceCreated=$true
    $ready=$false
    for($n=0;$n -lt 120;$n++){
        try{
            Release-Docker @('exec',$source,'sh','-c','test "$(cat /proc/1/comm)" = postgres && pg_isready -q -U postgres -d commerce')|Out-Null
            $ready=$true;break
        }catch{Start-Sleep -Milliseconds 500}
    }
    if(-not $ready){throw 'Disposable source PostgreSQL did not become ready.'}
    Release-Docker @('cp',(Join-Path $sourceRoot 'testdata/commerce.sql'),"${source}:/tmp/commerce.sql")|Out-Null
    Release-Docker @('exec',$source,'psql','-X','-U','postgres','-d','commerce','-v','ON_ERROR_STOP=1','-f','/tmp/commerce.sql')|Out-Null
    $count=Release-Docker @('exec',$source,'psql','-X','-At','-U','postgres','-d','commerce','-c','SELECT count(*) FROM order_items')
    if($count -ne '100000'){throw 'Synthetic commerce fixture is incomplete.'}
    Release-Docker @('exec',$source,'pg_dump','-U','postgres','-d','commerce','-Fc','-f','/tmp/commerce.dump')|Out-Null
    $archive=Join-Path $testRoot 'backups/commerce.dump'
    Release-Docker @('cp',"${source}:/tmp/commerce.dump","$archive.partial")|Out-Null
    [IO.File]::Move("$archive.partial",$archive)
    [IO.File]::WriteAllText((Join-Path $testRoot 'backups/bad.dump'),'synthetic invalid archive')
    $proof.archive_sha256=(Get-FileHash $archive).Hash.ToLowerInvariant()
    $proof.archive_bytes=(Get-Item $archive).Length
    Release-Docker @('rm','--force','--volumes',$source)|Out-Null
    $sourceCreated=$false
    Release-Docker @('compose','up','--no-build','--pull','never','-d','--wait','--wait-timeout','120') 150|Out-Null
    foreach($service in @('api','worker')){
        if((Release-Service $service).image -cne $imageID){throw 'Synthetic installation image identity changed.'}
    }
    # A healthy API precedes the first worker heartbeat; wait for that separate
    # signal without starting the measured soak with a known offline worker.
    . (Join-Path $testRoot 'scripts/api-session.ps1')
    $base="http://127.0.0.1:$Port"
    $session=New-RestivraSession -Base $base -EnvFile (Join-Path $testRoot '.env')
    $online=$false
    for($n=0;$n -lt 30;$n++){
        $h=Invoke-RestMethod "$base/api/health" -WebSession $session.WebSession -TimeoutSec 5
        if($h.worker_online){$online=$true;break}
        Start-Sleep -Seconds 1
    }
    if(-not $online){throw 'Synthetic worker did not become online.'}
    & (Join-Path $testRoot 'scripts/pilot-soak.ps1') -ProjectDirectory $testRoot -ComposeProject $project -Base $base -Minutes $Minutes -EvidenceDirectory (Join-Path $testRoot 'evidence')
    foreach($service in @('api','worker')){
        if((Release-Service $service).image -cne $imageID){throw 'Soak did not retain its fixed application image.'}
    }
    $support=Invoke-RestMethod "$base/api/support" -WebSession $session.WebSession -TimeoutSec 10
    $support|ConvertTo-Json -Depth 12|Set-Content (Join-Path $testRoot 'support-final.json')
    $proof.status='passed'
} finally {
    if($sourceCreated){try{Release-Docker @('rm','--force','--volumes',$source)|Out-Null}catch{Write-Warning 'Disposable source cleanup needs review.'}}
    $proof.finished_at=[datetime]::UtcNow.ToString('o')
    Release-JSON (Join-Path $testRoot 'soak-wrapper-summary.json') $proof
    try{
        # Check ownership before cleanup, and retain every data volume.
        $null=Release-Service db -AllowMissing
        Release-Docker @('compose','down') 90|Out-Null
    }catch{Write-Warning 'Synthetic soak project cleanup needs review.'}
    $script:ReleaseRoot=$sourceRoot;$env:COMPOSE_PROJECT_NAME=$oldProject;$env:RESTIVRA_PORT=$oldPort
    Write-Host "Synthetic soak evidence and private configuration retained under $testRoot"
}
