#Requires -Version 7.2
[CmdletBinding(DefaultParameterSetName='Update')]
param(
    [Parameter(Mandatory,ParameterSetName='Update')][string]$Version,
    [Parameter(ParameterSetName='Update')][switch]$Adopt,
    [Parameter(Mandatory,ParameterSetName='Rollback')][switch]$Rollback,
    [Parameter(Mandatory,ParameterSetName='Recover')][switch]$Recover,
    [ValidateRange(20,300)][int]$HealthTimeoutSeconds=120
)
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'release-ops.ps1')
. (Join-Path $PSScriptRoot 'release-recovery.ps1')
Release-LocalEngine
$operationLock=Release-Lock
$appsStopped=$false
$previous=$null
$checkpoint=$null
$statePath=Join-Path $script:ReleaseRoot 'private/deployment.json'
$journalPath=Join-Path $script:ReleaseRoot 'private/deployment-journal.json'
try {
    $state=if(Test-Path $statePath){Release-ReadJSON $statePath}else{$null}
    $journal=if(Test-Path $journalPath){Release-ReadJSON $journalPath}else{$null}
    if($Recover){Release-Recover $journal $state $statePath $journalPath $HealthTimeoutSeconds;return}
    if($journal -and $journal.status -eq 'in_progress'){throw 'An interrupted operation needs review. Use update.ps1 -Recover for supported journals; see docs/UPDATING.md.'}
    $api=Release-Service 'api';$worker=Release-Service 'worker';$db=Release-Service 'db'
    if(-not $api.running -or -not $worker.running -or -not $db.running -or $api.paused -or $worker.paused -or $db.paused){throw 'All three services must be running and unpaused before an update.'}
    if($api.image -ne $worker.image){throw 'API and worker image drift detected.'}
    $installation=Release-SQL 'SELECT id FROM installation'
    if($Rollback){
        if(-not $state -or -not $state.previous){throw 'No previous deployment is available.'}
        $target=$state.previous
        if($target.manifest){
            if($target.image -ne $target.manifest.image){throw 'Previous deployment manifest/image mismatch.'}
            $null=Release-VerifyImage $target.manifest -Pull
        }
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
        $previous=[pscustomobject]@{manifest=$null;image=$savedTag;image_id=$api.image;metadata_contract=1;binary_sha256=$manifest.binary_sha256}
    }else{throw 'Unmanaged installation. Explicit -Adopt is required for a binary-identical source installation.'}
    if((Release-ImageID $previous.image) -ne $api.image){throw 'The current rollback image reference changed or is unavailable.'}
    if($previous.image -eq $target.image){Write-Host 'Already running the requested release.';return}
    $config=(Release-Docker @('compose','config','--format','json'))|ConvertFrom-Json
    Release-NoForeignOneoffs $config.name
    Release-Idle
    Release-Docker @('compose','exec','-T','worker','restivra','doctor','--role','worker') 60|Out-Null
    $checkpoint=Join-Path $script:ReleaseRoot ('private/checkpoints/'+[datetime]::UtcNow.ToString('yyyyMMddTHHmmss')+'-'+[guid]::NewGuid().ToString('N'))
    New-RestivraPrivateDirectory $checkpoint
    Copy-Item -LiteralPath (Join-Path $script:ReleaseRoot '.env') -Destination (Join-Path $checkpoint '.env')
    Copy-Item -LiteralPath (Join-Path $script:ReleaseRoot 'private/sources.json') -Destination (Join-Path $checkpoint 'sources.json')
    $config=(Release-Docker @('compose','config','--format','json'))|ConvertFrom-Json
    $mode=if(-not $state){'adopt'}elseif($Rollback){'rollback'}else{'update'}
    $journal=[pscustomobject]@{format='restivra.operation.v2';status='in_progress';phase='prepared';mode=$mode;operation_id=[guid]::NewGuid().ToString('N');root=$script:ReleaseRoot;project=$config.name;database_container_id=$db.id;previous=$previous;target=$target;checkpoint=$checkpoint;installation=$installation;deployment_before=$state;api_selector_before=$config.services.api.image;worker_selector_before=$config.services.worker.image}
    Release-JSON $journalPath $journal
    $appsStopped=$true
    Release-StopService 'api'
    Release-Idle
    Release-StopService 'worker'
    Release-Idle
    Release-Phase $journal $journalPath 'apps_stopped'
    Write-Host 'Exporting and test-restoring private metadata before switching images...'
    & (Join-Path $PSScriptRoot 'metadata-backup.ps1') -OutputPath (Join-Path $checkpoint 'metadata.dump')
    # Separate process: metadata-check deliberately exits with an explicit status.
    $check=Invoke-RestivraNative -FilePath (Get-Process -Id $PID).Path -WorkingDirectory $script:ReleaseRoot -Arguments @('-NoProfile','-File',(Join-Path $PSScriptRoot 'metadata-check.ps1'),'-Archive',(Join-Path $checkpoint 'metadata.dump'),'-ReportPath',(Join-Path $checkpoint 'restore-check.json')) -TimeoutSeconds 210
    if($check.ExitCode -ne 0){throw 'Metadata restore verification failed; current app will be restarted.'}
    Release-Phase $journal $journalPath 'checkpoint_verified'
    Release-Start $target.image $HealthTimeoutSeconds -OperationID $journal.operation_id -OnAPIReady {Release-Phase $journal $journalPath 'api_ready'}
    Release-Phase $journal $journalPath 'services_healthy'
    Release-ProtectPrivate
    if((Release-Service 'db').id -ne $db.id){throw 'Database container unexpectedly changed.'}
    Release-SetImage $target.image
    Release-Phase $journal $journalPath 'selector_written'
    Release-JSON $statePath @{format='restivra.deployment.v1';installation=$installation;current=$target;previous=$previous;checkpoint=$checkpoint}
    Release-JSON $journalPath @{status='completed';current=$target.image;previous=$previous.image;checkpoint=$checkpoint}
    $appsStopped=$false
    Write-Host 'Deployment healthy. Previous image and verified private checkpoint retained. No metadata was overwritten.'
} catch {
    $failure=$_.Exception.Message
    if($appsStopped -and $previous){
        Write-Host 'Deployment did not complete; restarting the previous application image...'
        try {
            Release-Phase $journal $journalPath 'recovering_previous'
            # Stop a candidate worker before restarting the old API.
            Release-StopService 'api'
            Release-StopService 'worker'
            Release-CleanupDiagnostics $journal
            Release-Start $previous.image $HealthTimeoutSeconds -OperationID $journal.operation_id
            Release-SetImage $previous.image
            if($state){Release-JSON $statePath $state}
            Release-JSON $journalPath @{status='rolled_back';image=$previous.image;checkpoint=$checkpoint;metadata_restored=$false}
            Write-Host 'Previous application recovered. Database contents and credentials retained.'
        }catch{throw "Recovery also failed. Keep services/data intact and inspect private/deployment-journal.json and docs/UPDATING.md. Original failure: $failure"}
    }
    throw $failure
} finally { Remove-Item Env:RESTIVRA_IMAGE -ErrorAction SilentlyContinue; $operationLock.Dispose() }
