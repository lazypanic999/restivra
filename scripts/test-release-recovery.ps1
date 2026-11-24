#Requires -Version 7.2
# Real process interruption in a new synthetic installation. Never accepts an
# existing root, never restores metadata, never prunes the user's Docker engine.
param(
    [ValidateRange(1024,65535)][int]$Port=27481,
    [string]$FromVersion='v0.1.0-pilot.2',
    [string]$ToVersion='v0.1.0-pilot.3'
)
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'release-ops.ps1')
Release-LocalEngine
$sourceRoot=$script:ReleaseRoot
$suffix=[guid]::NewGuid().ToString('N')
$testRoot=Join-Path $sourceRoot "work/recovery-$suffix"
$null=New-Item -ItemType Directory $testRoot
Copy-Item (Join-Path $sourceRoot 'compose.yaml') $testRoot
Copy-Item (Join-Path $sourceRoot 'scripts') (Join-Path $testRoot 'scripts') -Recurse
$oldProject=$env:COMPOSE_PROJECT_NAME;$oldPort=$env:RESTIVRA_PORT
$env:COMPOSE_PROJECT_NAME="restivra-recovery-$($suffix.Substring(0,12))";$env:RESTIVRA_PORT="$Port"
$script:ReleaseRoot=$testRoot
& (Join-Path $testRoot 'scripts/setup.ps1')
[IO.File]::AppendAllText((Join-Path $testRoot '.env'),"`nCOMPOSE_PROJECT_NAME=$($env:COMPOSE_PROJECT_NAME)`nRESTIVRA_PORT=$Port`n")
$journalPath=Join-Path $testRoot 'private/deployment-journal.json'
$statePath=Join-Path $testRoot 'private/deployment.json'
$children=[Collections.Generic.List[object]]::new()
$invokePath=Join-Path $testRoot 'invoke-update.ps1'
@'
param([string]$Version)
try { & (Join-Path $PSScriptRoot 'scripts/update.ps1') -Version $Version; exit 0 }
catch { Write-Output ('UPDATER_ERROR: '+$_.Exception.Message); exit 1 }
'@ | Set-Content $invokePath
$proof=[ordered]@{status='failed';synthetic_data=$true;from=$FromVersion;to=$ToVersion;checks=@();started_at=[datetime]::UtcNow.ToString('o')}
function Assert-Recovery($Condition,[string]$Message){if(-not $Condition){throw $Message}}
function Spawn([string]$File,[string[]]$Arguments) {
    $p=[Diagnostics.Process]::new();$p.StartInfo.FileName=$File
    $p.StartInfo.WorkingDirectory=$testRoot;$p.StartInfo.UseShellExecute=$false;$p.StartInfo.CreateNoWindow=$true
    $p.StartInfo.RedirectStandardOutput=$true;$p.StartInfo.RedirectStandardError=$true
    foreach($arg in $Arguments){$p.StartInfo.ArgumentList.Add($arg)}
    $null=$p.Start()
    $handle=[pscustomobject]@{Process=$p;Output=[Restivra.NativeIO]::ReadAsync($p.StandardOutput,65536);Errors=$p.StandardError.BaseStream.CopyToAsync([IO.Stream]::Null)}
    $children.Add($handle);return $handle
}
function Kill-OwnChild($Handle){if($Handle -and -not $Handle.Process.HasExited){$Handle.Process.Kill($true);$null=$Handle.Process.WaitForExit(10000)}}
function Start-Update {Spawn (Get-Process -Id $PID).Path @('-NoProfile','-File',$invokePath,'-Version',$ToVersion)}
function Wait-Phase($Handle,[string]$Phase,[switch]$Doctor) {
    $deadline=[datetime]::UtcNow.AddMinutes(3)
    while([datetime]::UtcNow -lt $deadline){
        if($Handle.Process.HasExited){
            $reason=''
            if($Handle.Output.Wait(3000)){$reason=($Handle.Output.GetAwaiter().GetResult().Text -split '\r?\n'|Where-Object{$_ -like 'UPDATER_ERROR:*'}|Select-Object -Last 1)}
            throw "Updater exited before expected phase $Phase. $reason"
        }
        if(Test-Path $journalPath){
            try{$j=Get-Content $journalPath -Raw|ConvertFrom-Json}catch{$j=$null}
            if($j -and $j.status -eq 'in_progress' -and $j.phase -eq $Phase){
                if(-not $Doctor -or (Release-Docker @('ps','--quiet','--filter',"label=io.restivra.operation=$($j.operation_id)"))){return $j}
            }
        }
        Start-Sleep -Milliseconds 100
    }
    throw "Expected updater phase $Phase was not observed."
}
function Fingerprint {Release-SQL "SELECT json_build_object('projects',(SELECT md5(coalesce(jsonb_agg(to_jsonb(p) ORDER BY p.id)::text,'')) FROM projects p),'jobs',(SELECT md5(coalesce(jsonb_agg(to_jsonb(j) ORDER BY j.id)::text,'')) FROM jobs j),'attempts',(SELECT md5(coalesce(jsonb_agg(to_jsonb(a) ORDER BY a.job_id,a.number)::text,'')) FROM job_attempts a),'installation',(SELECT id FROM installation))::text"}
function Check-Preserved {
    Assert-Recovery ((Fingerprint) -ceq $script:baseline) 'Synthetic metadata/history changed during recovery.'
    Assert-Recovery ((Release-Service db).id -ceq $script:databaseID) 'Metadata container changed during recovery.'
    $settings=(Get-Content (Join-Path $testRoot '.env')|Where-Object{$_ -notmatch '^RESTIVRA_IMAGE='}) -join "`n"
    Assert-Recovery ($settings -ceq $script:settings) 'Recovery changed credentials/settings.'
    Assert-Recovery ((Get-FileHash (Join-Path $testRoot 'private/sources.json')).Hash -eq $script:profilesHash) 'Recovery changed source profiles.'
}
function Assert-Refused([scriptblock]$Action,[string]$Expected) {
    $api=Release-Service api -AllowMissing;$worker=Release-Service worker -AllowMissing
    $rejected=$false
    try{& $Action}catch{if($_.Exception.Message -match $Expected){$rejected=$true}else{throw}}
    Assert-Recovery $rejected 'Unsafe recovery was not rejected.'
    $afterAPI=Release-Service api -AllowMissing;$afterWorker=Release-Service worker -AllowMissing
    Assert-Recovery ($api.id -eq $afterAPI.id -and $worker.id -eq $afterWorker.id -and $api.running -eq $afterAPI.running -and $worker.running -eq $afterWorker.running) 'Rejected recovery changed application containers.'
}
try{
    & (Join-Path $testRoot 'scripts/install.ps1') -Version $FromVersion
    & (Join-Path $testRoot 'scripts/pilot-smoke.ps1') -Base "http://127.0.0.1:$Port"
    $script:baseline=Fingerprint;$script:databaseID=(Release-Service db).id
    $script:settings=(Get-Content (Join-Path $testRoot '.env')|Where-Object{$_ -notmatch '^RESTIVRA_IMAGE='}) -join "`n"
    $script:profilesHash=(Get-FileHash (Join-Path $testRoot 'private/sources.json')).Hash
    # Hold only the synthetic attempt table. Idle checks still run; pg_dump
    # blocks, giving a deterministic real mid-export interruption point.
    $gateName="restivra-recovery-gate-$suffix"
    $gate=Spawn docker @('compose','exec','-T','db','env',"PGAPPNAME=$gateName",'psql','-X','-U','restivra','-d','restivra','-v','ON_ERROR_STOP=1','-c',"BEGIN; LOCK TABLE job_attempts IN ACCESS EXCLUSIVE MODE; SELECT pg_sleep(120)")
    try{
        $locked=$false
        for($n=0;$n -lt 100;$n++){
            if((Release-SQL "SELECT count(*) FROM pg_locks l JOIN pg_stat_activity a USING(pid) WHERE a.application_name='$gateName' AND l.relation='public.job_attempts'::regclass AND l.mode='AccessExclusiveLock' AND l.granted") -eq '1'){$locked=$true;break}
            Start-Sleep -Milliseconds 100
        }
        Assert-Recovery $locked 'Synthetic metadata export gate was not acquired.'
        $updater=Start-Update
        $interrupted=Wait-Phase $updater 'apps_stopped'
        Kill-OwnChild $updater
    }finally{
        Release-SQL "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE application_name='$gateName'"|Out-Null
        Kill-OwnChild $gate
    }
    Assert-Recovery ((Get-Content $journalPath -Raw|ConvertFrom-Json).status -eq 'in_progress') 'Abrupt exit did not retain the operation journal.'
    & (Join-Path $testRoot 'scripts/update.ps1') -Recover
    Check-Preserved
    Assert-Recovery ((Get-Content $statePath -Raw|ConvertFrom-Json).current.manifest.version -eq $FromVersion) 'Recovery did not restore the previous release.'
    $proof.checks+='actual_updater_process_kill_during_metadata_export_recovers_without_restoring_database'
    # Block the candidate's diagnostic command after the API is actually healthy.
    # This is a Compose-only fixture, not an updater test bypass or fake Docker.
    $target=Release-ReadManifest $ToVersion
    $override=@'
services:
  worker:
    environment:
      QA_IMAGE: ${RESTIVRA_IMAGE}
    entrypoint:
      - /bin/sh
      - -c
      - 'if [ "$1" = doctor ] && [ "$QA_IMAGE" = "FAIL_IMAGE" ]; then sleep 300; fi; exec restivra "$@"'
      - restivra-test-entry
'@.Replace('FAIL_IMAGE',$target.image).Replace('$1','$$1').Replace('$QA_IMAGE','$$QA_IMAGE').Replace('$@','$$@')
    [IO.File]::WriteAllText((Join-Path $testRoot 'compose.override.yaml'),$override)
    $updater=Start-Update
    $interrupted=Wait-Phase $updater 'api_ready' -Doctor
    Assert-Recovery ((Release-Service api).image -eq $interrupted.target.image_id) 'Candidate API was not actually started before interruption.'
    Kill-OwnChild $updater
    # Identity and ownership failures must be refused before stopping this API.
    foreach($field in @('root','project','installation','database_container_id')){
        $bad=$interrupted|ConvertTo-Json -Depth 20|ConvertFrom-Json
        $bad.$field=if($field -eq 'root'){Join-Path $testRoot 'foreign'}elseif($field -eq 'project'){'another-compose-project'}elseif($field -eq 'installation'){'f'*32}else{'e'*64}
        Release-JSON $journalPath $bad
        Assert-Refused {& (Join-Path $testRoot 'scripts/update.ps1') -Recover} 'identity|different|original healthy|installation'
    }
    Release-JSON $journalPath $interrupted
    $bad=$interrupted|ConvertTo-Json -Depth 20|ConvertFrom-Json
    $bad.previous.image_id='sha256:'+('f'*64)
    Release-JSON $journalPath $bad
    Assert-Refused {& (Join-Path $testRoot 'scripts/update.ps1') -Recover} 'image changed'
    $bad=$interrupted|ConvertTo-Json -Depth 20|ConvertFrom-Json
    $bad.target.manifest.metadata_contract=2
    Release-JSON $journalPath $bad
    Assert-Refused {& (Join-Path $testRoot 'scripts/update.ps1') -Recover} 'Unsupported or invalid'
    Release-JSON $journalPath $interrupted
    $savedSelector=(Get-Content $statePath -Raw|ConvertFrom-Json).current.image
    Release-SetImage 'restivra:unexpected-selector'
    try{Assert-Refused {& (Join-Path $testRoot 'scripts/update.ps1') -Recover} 'Compose image drift'}finally{Release-SetImage $savedSelector}
    $candidateAPI=Release-Service api
    Release-Docker @('pause',$candidateAPI.id)|Out-Null
    try{Assert-Refused {& (Join-Path $testRoot 'scripts/update.ps1') -Recover} 'paused application'}finally{Release-Docker @('unpause',$candidateAPI.id)|Out-Null}
    $proof.checks+='changed_rollback_image_incompatible_contract_selector_drift_and_paused_app_refused'
    $queueID=[guid]::NewGuid().ToString('N')
    Release-SQL "INSERT INTO jobs(id,project_id,project_name,backup_pattern,config,status,trigger,retry_at) SELECT '$queueID',id,name,backup_pattern,config,'queued','manual',now()+interval '1 hour' FROM projects ORDER BY id LIMIT 1"|Out-Null
    try{Assert-Refused {& (Join-Path $testRoot 'scripts/update.ps1') -Recover} 'Queued/running jobs'}finally{Release-SQL "DELETE FROM jobs WHERE id='$queueID' AND status='queued'"|Out-Null}
    $proof.checks+='foreign_root_project_installation_database_and_queued_work_refused_without_container_changes'
    $foreignName="restivra-recovery-foreign-$suffix"
    $foreign=Release-Docker @('compose','run','--detach','--no-deps','--name',$foreignName,'--entrypoint','/bin/sh','worker','-c','sleep 120')
    try{
        Assert-Refused {& (Join-Path $testRoot 'scripts/update.ps1') -Recover} 'one-off operation'
        Assert-Recovery ((Release-Docker @('inspect',$foreignName,'--format','{{.State.Running}}')) -eq 'true') 'Foreign one-off command was stopped.'
    }finally{Release-Docker @('rm','--force',$foreignName)|Out-Null}
    $proof.checks+='unrelated_oneoff_command_is_neither_stopped_nor_deleted'
    $diagnostics=Release-Docker @('ps','--all','--quiet','--filter',"label=io.restivra.operation=$($interrupted.operation_id)")
    Assert-Recovery ([bool]$diagnostics) 'Owned diagnostic did not remain for the real recovery cleanup check.'
    # Cover Compose interruption between removing and recreating an app service.
    $missingWorker=Release-Service worker
    Assert-Recovery (-not $missingWorker.running) 'Candidate worker unexpectedly started during the diagnostic gate.'
    Release-Docker @('rm',$missingWorker.id)|Out-Null
    & (Join-Path $testRoot 'scripts/update.ps1') -Recover
    Check-Preserved
    $recovered=Get-Content $journalPath -Raw|ConvertFrom-Json
    Assert-Recovery ($recovered.status -eq 'recovered' -and -not $recovered.metadata_restored) 'Recovery was not recorded safely.'
    Assert-Recovery (-not (Release-Docker @('ps','--all','--quiet','--filter',"label=io.restivra.operation=$($interrupted.operation_id)"))) 'Owned diagnostic container was not cleaned up.'
    Assert-Refused {& (Join-Path $testRoot 'scripts/update.ps1') -Recover} 'No supported interrupted'
    $proof.checks+='actual_updater_process_kill_after_candidate_api_health_recovers_and_cleans_only_owned_diagnostic'
    $proof.checks+='missing_regular_worker_container_is_recreated_with_original_data_volume'
    & (Join-Path $testRoot 'scripts/pilot-check.ps1') -Base "http://127.0.0.1:$Port"
    $proof.checks+='previous_release_authenticated_health_and_worker_checks_pass_after_recovery'
    $proof.status='passed'
}finally{
    $childNumber=0
    foreach($child in $children){
        $childNumber++
        try{Kill-OwnChild $child}catch{}
        try{if($child.Output.Wait(3000)){[IO.File]::WriteAllText((Join-Path $testRoot "child-$childNumber.stdout.log"),$child.Output.GetAwaiter().GetResult().Text)}}catch{}
        $child.Process.Dispose()
    }
    $proof.finished_at=[datetime]::UtcNow.ToString('o')
    Release-JSON (Join-Path $testRoot 'recovery-summary.json') $proof
    try{Release-Docker @('compose','down','--remove-orphans') 90|Out-Null}catch{Write-Warning 'Recovery test cleanup needs review.'}
    $script:ReleaseRoot=$sourceRoot;$env:COMPOSE_PROJECT_NAME=$oldProject;$env:RESTIVRA_PORT=$oldPort
    Write-Host "Recovery evidence retained under $testRoot"
}
