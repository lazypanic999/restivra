# Requires release-ops.ps1 and the caller's exclusive operation lock.
function Release-Recover($Journal,$State,[string]$StatePath,[string]$JournalPath,[int]$HealthTimeoutSeconds) {
    if(-not $Journal -or $Journal.status -ne 'in_progress' -or $Journal.format -ne 'restivra.operation.v2'){
        throw 'No supported interrupted operation was found. Older journals require the manual recovery procedure.'
    }
    if($Journal.mode -cnotin @('update','rollback','adopt')){throw 'Unsupported interrupted operation mode.'}
    if($Journal.operation_id -cnotmatch '^[a-f0-9]{32}$' -or $Journal.installation -cnotmatch '^[a-f0-9]{32}$' -or
       $Journal.database_container_id -cnotmatch '^[a-f0-9]{64}$' -or -not (Release-OwnDirectory $Journal.root)){
        throw 'Interrupted operation identity does not match this installation.'
    }
    $config=(Release-Docker @('compose','config','--format','json'))|ConvertFrom-Json
    if($config.name -cne $Journal.project){throw 'Interrupted operation belongs to a different Compose project.'}
    Release-NoForeignOneoffs $Journal.project $Journal.operation_id
    $db=Release-Service 'db'
    if(-not $db.running -or $db.paused -or $db.id -cne $Journal.database_container_id){throw 'Restore the original healthy metadata database container before recovery.'}
    $installation=Release-SQL 'SELECT id FROM installation'
    if($installation -cne $Journal.installation -or ($State -and $State.installation -cne $installation)){
        throw 'Interrupted operation refers to a different metadata installation.'
    }
    $schema=Release-SQL "SELECT to_regclass('public.projects') IS NOT NULL AND to_regclass('public.jobs') IS NOT NULL AND to_regclass('public.workers') IS NOT NULL AND to_regclass('public.installation') IS NOT NULL AND to_regclass('public.job_attempts') IS NOT NULL"
    if($schema -ne 't'){throw 'Metadata schema needs manual recovery; application-image recovery cannot replace missing data.'}
    $previous=$Journal.previous
    $target=$Journal.target
    # Validate every record and selector before stopping anything. A cached
    # official digest can be verified and recovered without registry access.
    Release-VerifyRecord $previous
    Release-ValidateRecord $target
    if(-not $target.manifest -or $target.manifest.metadata_contract -ne 1){throw 'Unsupported recovery metadata contract.'}
    if($State -and ($State.format -ne 'restivra.deployment.v1' -or $State.current.image_id -cnotin @($previous.image_id,$target.image_id))){throw 'Current deployment state needs manual review.'}
    $selectors=@($previous.image,$target.image,$Journal.api_selector_before,$Journal.worker_selector_before)
    if($config.services.api.image -cnotin $selectors -or $config.services.worker.image -cnotin $selectors){throw 'Compose image drift needs manual review before recovery.'}
    foreach($service in @('api','worker')){
        $container=Release-Service $service -AllowMissing
        if($container -and ($container.paused -or $container.image -cnotin @($previous.image_id,$target.image_id))){throw 'Unexpected or paused application container needs manual review.'}
    }
    $before=$Journal.deployment_before
    if($before){
        if($before.format -ne 'restivra.deployment.v1' -or $before.installation -cne $installation -or $before.current.image_id -cne $previous.image_id -or $before.current.image -cne $previous.image){throw 'Original deployment snapshot is inconsistent.'}
        if($Journal.mode -eq 'adopt'){throw 'Adoption journal unexpectedly contains a managed deployment snapshot.'}
        $before.current=$previous
    }elseif($Journal.mode -ne 'adopt' -or $previous.manifest -or $previous.binary_sha256 -cne $target.manifest.binary_sha256){
        throw 'Original deployment snapshot is missing; manual recovery review is required.'
    }
    Release-Idle
    Release-Phase $Journal $JournalPath 'recovering_previous'
    Release-StopService 'api'
    Release-Idle
    Release-StopService 'worker'
    Release-Idle
    # Only diagnostic containers explicitly labelled by THIS interrupted
    # operation are disposable. Never touch arbitrary one-off worker commands.
    Release-CleanupDiagnostics $Journal
    Release-Start $previous.image $HealthTimeoutSeconds -OperationID $Journal.operation_id
    if((Release-Service db).id -cne $db.id){throw 'Metadata database container changed during recovery.'}
    Release-SetImage $previous.image
    $restored=if($before){$before}else{@{format='restivra.deployment.v1';installation=$installation;current=$previous;previous=$null;checkpoint=$Journal.checkpoint}}
    Release-JSON $StatePath $restored
    Release-JSON $JournalPath @{format='restivra.operation.v2';status='recovered';operation_id=$Journal.operation_id;image=$previous.image;checkpoint=$Journal.checkpoint;metadata_restored=$false;finished_at=[datetime]::UtcNow.ToString('o')}
    Write-Host 'Previous application recovered and checked. PostgreSQL contents and credentials were preserved.'
}

function Release-CleanupDiagnostics($Journal) {
    $owned=(Release-Docker @('ps','--all','--quiet','--filter',"label=com.docker.compose.project=$($Journal.project)",'--filter',"label=io.restivra.operation=$($Journal.operation_id)")) -split '\r?\n'|Where-Object{$_}
    foreach($id in $owned){
        $owner=Release-Docker @('inspect',$id,'--format','{{index .Config.Labels "com.docker.compose.project.working_dir"}}')
        $service=Release-Docker @('inspect',$id,'--format','{{index .Config.Labels "com.docker.compose.service"}}')
        $oneoff=Release-Docker @('inspect',$id,'--format','{{index .Config.Labels "com.docker.compose.oneoff"}}')
        if(-not (Release-OwnDirectory $owner) -or $service -ne 'worker' -or $oneoff -ne 'True'){throw 'Operation-labelled container ownership is inconsistent.'}
        Release-Docker @('rm','--force',$id)|Out-Null
    }
}
