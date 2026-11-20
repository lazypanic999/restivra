#Requires -Version 7.2
[CmdletBinding(DefaultParameterSetName='Update')]
param(
    [Parameter(Mandatory,ParameterSetName='Update')][string]$Version,
    [Parameter(ParameterSetName='Update')][switch]$Adopt,
    [Parameter(Mandatory,ParameterSetName='Rollback')][switch]$Rollback,
    [ValidateRange(20,300)][int]$HealthTimeoutSeconds=120
)
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'release-ops.ps1')
Release-LocalEngine
$operationLock=Release-Lock
$appsStopped=$false
$previous=$null
$switched=$false
$checkpoint=$null
$statePath=Join-Path $script:ReleaseRoot 'private/deployment.json'
$journalPath=Join-Path $script:ReleaseRoot 'private/deployment-journal.json'
try {
    $state=if(Test-Path $statePath){Get-Content $statePath -Raw|ConvertFrom-Json}else{$null}
    $journal=if(Test-Path $journalPath){Get-Content $journalPath -Raw|ConvertFrom-Json}else{$null}
    if($journal -and $journal.status -eq 'in_progress'){throw 'An interrupted operation needs review. See docs/UPDATING.md; no new update was started.'}
    $api=Release-Service 'api';$worker=Release-Service 'worker';$db=Release-Service 'db'
    if(-not $api.running -or -not $worker.running -or -not $db.running){throw 'All three services must be running before an update.'}
    if($api.image -ne $worker.image){throw 'API and worker image drift detected.'}
    $installation=Release-SQL 'SELECT id FROM installation'
    if($Rollback){
        if(-not $state -or -not $state.previous){throw 'No previous deployment is available.'}
        $target=$state.previous
        if($target.manifest){$null=Release-VerifyImage $target.manifest -Pull}
        if((Release-ImageID $target.image) -ne $target.image_id){throw 'Previous image changed or is unavailable.'}
    }else{
        $manifest=Release-ReadManifest $Version
        $imageID=Release-VerifyImage $manifest -Pull
        $target=[pscustomobject]@{manifest=$manifest;image=$manifest.image;image_id=$imageID}
    }
    if($state){
        if($state.format -ne 'restivra.deployment.v1' -or $state.installation -ne $installation -or $state.current.image_id -ne $api.image){throw 'Deployment state does not match this installation.'}
        $previous=$state.current
        if($previous.manifest -and $target.manifest -and $previous.manifest.metadata_contract -ne $target.manifest.metadata_contract){throw 'Metadata contract differs; automatic updates are not supported.'}
        $config=(Release-Docker @('compose','config','--format','json'))|ConvertFrom-Json
        if($config.services.api.image -ne $previous.image -or $config.services.worker.image -ne $previous.image){throw 'Compose image drift detected.'}
    }elseif($Adopt -and -not $Rollback){
        if((Release-BinarySHA $api.image) -ne $manifest.binary_sha256){throw 'Source adoption requires an identical application binary; review migrations manually.'}
        $savedTag='restivra:rollback-'+[guid]::NewGuid().ToString('N')
        Release-Docker @('tag',$api.image,$savedTag)|Out-Null
        $previous=[pscustomobject]@{manifest=$null;image=$savedTag;image_id=$api.image}
    }else{throw 'Unmanaged installation. Explicit -Adopt is required for a binary-identical source installation.'}
    if($previous.image -eq $target.image){Write-Host 'Already running the requested release.';return}
    Release-Idle
    $checkpoint=Join-Path $script:ReleaseRoot ('private/checkpoints/'+[datetime]::UtcNow.ToString('yyyyMMddTHHmmss')+'-'+[guid]::NewGuid().ToString('N'))
    $null=New-Item -ItemType Directory $checkpoint
    Copy-Item -LiteralPath (Join-Path $script:ReleaseRoot '.env') -Destination (Join-Path $checkpoint '.env')
    Copy-Item -LiteralPath (Join-Path $script:ReleaseRoot 'private/sources.json') -Destination (Join-Path $checkpoint 'sources.json')
    Release-JSON $journalPath @{status='in_progress';previous=$previous;target=$target;checkpoint=$checkpoint;installation=$installation}
    $appsStopped=$true
    Release-Docker @('compose','stop','api') | Out-Null
    Release-Idle
    Release-Docker @('compose','stop','worker') 90 | Out-Null
    Release-Idle
    Write-Host 'Exporting and test-restoring private metadata before switching images...'
    & (Join-Path $PSScriptRoot 'metadata-backup.ps1') -OutputPath (Join-Path $checkpoint 'metadata.dump')
    # Separate process: metadata-check deliberately exits with an explicit status.
    $check=Invoke-RestivraNative -FilePath (Get-Process -Id $PID).Path -WorkingDirectory $script:ReleaseRoot -Arguments @('-NoProfile','-File',(Join-Path $PSScriptRoot 'metadata-check.ps1'),'-Archive',(Join-Path $checkpoint 'metadata.dump'),'-ReportPath',(Join-Path $checkpoint 'restore-check.json')) -TimeoutSeconds 210
    if($check.ExitCode -ne 0){throw 'Metadata restore verification failed; current app will be restarted.'}
    $switched=$true
    Release-Start $target.image $HealthTimeoutSeconds
    if((Release-Service 'db').id -ne $db.id){throw 'Database container unexpectedly changed.'}
    Release-SetImage $target.image
    Release-JSON $statePath @{format='restivra.deployment.v1';installation=$installation;current=$target;previous=$previous;checkpoint=$checkpoint}
    Release-JSON $journalPath @{status='completed';current=$target.image;previous=$previous.image;checkpoint=$checkpoint}
    $appsStopped=$false
    Write-Host 'Deployment healthy. Previous image and verified private checkpoint retained. No metadata was overwritten.'
} catch {
    $failure=$_.Exception.Message
    if($appsStopped -and $previous){
        Write-Host 'Deployment did not complete; restarting the previous application image...'
        try {
            # Stop a candidate worker before restarting the old API.
            Release-Docker @('compose','stop','api','worker') 90|Out-Null
            Release-Start $previous.image $HealthTimeoutSeconds
            Release-SetImage $previous.image
            if($state){Release-JSON $statePath $state}
            Release-JSON $journalPath @{status='rolled_back';image=$previous.image;checkpoint=$checkpoint;metadata_restored=$false}
            Write-Host 'Previous application recovered. Database contents and credentials retained.'
        }catch{throw "Recovery also failed. Keep services/data intact and inspect private/deployment-journal.json and docs/UPDATING.md. Original failure: $failure"}
    }
    throw $failure
} finally { Remove-Item Env:RESTIVRA_IMAGE -ErrorAction SilentlyContinue; $operationLock.Dispose() }
