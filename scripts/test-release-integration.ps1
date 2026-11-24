#Requires -Version 7.2
# Creates its OWN unique Compose project, credentials, data volumes and synthetic
# backups under work/. Never points at an existing installation. Retains evidence.
param(
    [string]$FromVersion='v0.1.0-pilot.2',
    [Parameter(Mandatory)][string]$ToVersion,
    [ValidateRange(1024,65535)][int]$Port=27475
)
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'release-ops.ps1')
. (Join-Path $PSScriptRoot 'qa-environment.ps1')
Release-LocalEngine
$sourceRoot=$script:ReleaseRoot
$testID=[guid]::NewGuid().ToString('N')
$testRoot=Join-Path $sourceRoot "work/release-integration-$testID"
$null=New-Item -ItemType Directory -Force $testRoot
Copy-Item (Join-Path $sourceRoot 'compose.yaml') $testRoot
Copy-Item (Join-Path $sourceRoot 'scripts') (Join-Path $testRoot 'scripts') -Recurse
$qaEnvironment=Enter-RestivraQAEnvironment "restivra-release-$($testID.Substring(0,12))" $Port
$script:ReleaseRoot=$testRoot
& (Join-Path $testRoot 'scripts/setup.ps1')
$proof=[ordered]@{status='failed';from=$FromVersion;to=$ToVersion;synthetic_data=$true;platform='Windows/PowerShell/Linux-amd64-Docker';checks=@();started_at=[datetime]::UtcNow.ToString('o')}
function Assert-Release($Condition,[string]$Message){if(-not $Condition){throw $Message}}
function Fingerprint {
    Release-SQL "SELECT json_build_object('projects',(SELECT md5(coalesce(jsonb_agg(to_jsonb(p) ORDER BY p.id)::text,'')) FROM projects p),'jobs',(SELECT md5(coalesce(jsonb_agg(to_jsonb(j) ORDER BY j.id)::text,'')) FROM jobs j),'attempts',(SELECT md5(coalesce(jsonb_agg(to_jsonb(a) ORDER BY a.job_id,a.number)::text,'')) FROM job_attempts a),'installation',(SELECT id FROM installation))::text"
}
function Private-Settings {
    # Returned to memory only; not part of public evidence.
    ((Get-Content (Join-Path $testRoot '.env')|Where-Object{$_ -notmatch '^RESTIVRA_IMAGE='}) -join "`n")
}
function State {Get-Content (Join-Path $testRoot 'private/deployment.json') -Raw|ConvertFrom-Json}
function Check-Preserved {
    Assert-Release ((Fingerprint) -eq $script:baseline) 'Metadata definitions/history changed.'
    Assert-Release ((Private-Settings) -ceq $script:settings) 'Private settings changed.'
    Assert-Release ((Get-FileHash (Join-Path $testRoot 'private/sources.json')).Hash -eq $script:sourceHash) 'Source profiles changed.'
    Assert-Release ((Release-Service db).id -eq $script:databaseID) 'Metadata database container changed.'
}
try{
    & (Join-Path $testRoot 'scripts/install.ps1') -Version $FromVersion
    $proof.checks+='fresh_public_image_install'
    & (Join-Path $testRoot 'scripts/pilot-smoke.ps1') -Base "http://127.0.0.1:$Port"
    $proof.checks+='real_postgresql_pass_and_intentional_sql_failure'
    $script:baseline=Fingerprint;$script:settings=Private-Settings
    $script:sourceHash=(Get-FileHash (Join-Path $testRoot 'private/sources.json')).Hash
    $script:databaseID=(Release-Service db).id
    & (Join-Path $testRoot 'scripts/setup.ps1')
    Check-Preserved
    $proof.checks+='repeat_setup_preserves_credentials'
    & (Join-Path $testRoot 'scripts/update.ps1') -Version $ToVersion
    Check-Preserved
    Assert-Release ((State).current.manifest.version -eq $ToVersion) 'Target version not deployed.'
    $proof.checks+='compatible_update_preserves_all_metadata_and_database_container'
    . (Join-Path $testRoot 'scripts/api-session.ps1')
    $connection=New-RestivraSession -Base "http://127.0.0.1:$Port" -EnvFile (Join-Path $testRoot '.env')
    $targetReports=@()
    if($connection.Session.build){
        $manifest=Release-ReadManifest $ToVersion
        Assert-Release ($connection.Session.build.version -ceq $manifest.version -and $connection.Session.build.revision -ceq $manifest.revision) 'API runtime does not match the published manifest.'
        & (Join-Path $testRoot 'scripts/pilot-smoke.ps1')
        $latest=Get-ChildItem (Join-Path $testRoot 'work') -Directory -Filter 'pilot-*'|Sort-Object LastWriteTimeUtc -Descending|Select-Object -First 1
        $sample=Get-Content (Join-Path $latest.FullName 'summary.json') -Raw|ConvertFrom-Json
        foreach($entry in $sample.projects){
            $job=Invoke-RestMethod "http://127.0.0.1:$Port/api/jobs/$($entry.job_id)" -WebSession $connection.WebSession -TimeoutSec 10
            Assert-Release ($job.report.application_build.version -ceq $manifest.version -and $job.report.application_build.revision -ceq $manifest.revision) 'Target restore report did not record its published verifier build.'
            $targetReports+=$entry.job_id
        }
        # These legitimate new sample runs now join the preserved baseline:
        # old binaries may ignore an optional JSON field but must not erase it.
        $script:baseline=Fingerprint
        $proof.checks+='published_runtime_and_new_restore_reports_match_manifest_before_rollback'
    }
    & (Join-Path $testRoot 'scripts/update.ps1') -Rollback
    Check-Preserved
    Assert-Release ((State).current.manifest.version -eq $FromVersion) 'Explicit rollback did not restore the previous version.'
    $proof.checks+='explicit_rollback_preserves_all_metadata'
    & (Join-Path $testRoot 'scripts/update.ps1') -Version $ToVersion
    Check-Preserved
    $targetState=State
    # An intentionally deferred, synthetic queue row cannot be claimed by the
    # worker while we exercise the updater's active-job guard.
    $queueID=[guid]::NewGuid().ToString('N')
    Release-SQL "INSERT INTO jobs(id,project_id,project_name,backup_pattern,config,status,trigger,retry_at) SELECT '$queueID',id,name,backup_pattern,config,'queued','manual',now()+interval '1 hour' FROM projects ORDER BY id LIMIT 1"|Out-Null
    $apiBefore=(Release-Service api).id
    $rejected=$false
    try{& (Join-Path $testRoot 'scripts/update.ps1') -Version $FromVersion}catch{if($_.Exception.Message -match 'Queued/running jobs'){$rejected=$true}else{throw}}
    Assert-Release $rejected 'Queued work was not rejected.'
    Assert-Release ((Release-Service api).id -eq $apiBefore) 'Preflight rejection restarted the API.'
    Release-SQL "DELETE FROM jobs WHERE id='$queueID' AND status='queued'"|Out-Null
    Check-Preserved
    $proof.checks+='queued_work_rejected_before_stopping_services'
    # A real official candidate image exits on startup through a local Compose
    # override. No updater test hooks or mocked Docker commands are involved.
    $failImage=$targetState.previous.image
    $override=@'
services:
  api:
    environment:
      QA_IMAGE: ${RESTIVRA_IMAGE}
    entrypoint: ["/bin/sh", "-c"]
    command:
      - 'if [ "$$QA_IMAGE" = "FAIL_IMAGE" ]; then exit 42; fi; exec restivra serve --listen 0.0.0.0:7474'
'@.Replace('FAIL_IMAGE',$failImage)
    [IO.File]::WriteAllText((Join-Path $testRoot 'compose.override.yaml'),$override)
    $rejected=$false
    try{& (Join-Path $testRoot 'scripts/update.ps1') -Version $FromVersion -HealthTimeoutSeconds 20}catch{$rejected=$true}
    Assert-Release $rejected 'Broken startup incorrectly reported success.'
    $journal=Get-Content (Join-Path $testRoot 'private/deployment-journal.json') -Raw|ConvertFrom-Json
    Assert-Release ($journal.status -eq 'rolled_back' -and $journal.image -eq $targetState.current.image) 'Automatic recovery was not verified.'
    Assert-Release ((State).current.image -eq $targetState.current.image) 'Failed deployment changed managed state.'
    Check-Preserved
    $proof.checks+='real_failed_startup_automatically_recovers_previous_image_without_database_restore'
    & (Join-Path $testRoot 'scripts/pilot-check.ps1') -Base "http://127.0.0.1:$Port"
    $proof.checks+='authenticated_health_and_worker_diagnostics_after_recovery'
    if($targetReports.Count){
        $connection=New-RestivraSession -Base "http://127.0.0.1:$Port" -EnvFile (Join-Path $testRoot '.env')
        foreach($id in $targetReports){
            $job=Invoke-RestMethod "http://127.0.0.1:$Port/api/jobs/$id" -WebSession $connection.WebSession -TimeoutSec 10
            Assert-Release ($job.report.application_build.version -ceq $manifest.version -and $job.report.application_build.revision -ceq $manifest.revision) 'Older-binary rollback lost the new report provenance.'
        }
        $proof.checks+='new_report_provenance_survives_older_binary_rollback_and_failed_startup_recovery'
    }
    $proof.status='passed'
    $proof.metadata_fingerprint=$script:baseline|ConvertFrom-Json
    $proof.from_image=$targetState.previous.image
    $proof.to_image=$targetState.current.image
}finally{
    $proof.finished_at=[datetime]::UtcNow.ToString('o')
    Release-JSON (Join-Path $testRoot 'summary.json') $proof
    # Only the project's unique containers/networks are removed. Keep volumes,
    # checkpoints and evidence for review; never prune the engine or user data.
    try{Release-Docker @('compose','down') 90|Out-Null}catch{Write-Warning 'Test project cleanup needs review.'}
    Restore-RestivraQAEnvironment $qaEnvironment
    $script:ReleaseRoot=$sourceRoot
    Write-Host "Release integration evidence: $testRoot/summary.json"
}
