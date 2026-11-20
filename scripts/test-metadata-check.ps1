#Requires -Version 7.2
param([Parameter(Mandatory)][string]$MetadataArchive,[Parameter(Mandatory)][string]$ApplicationArchive)
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'native-command.ps1')
$root=Split-Path -Parent $PSScriptRoot
$evidence=Join-Path $root ('work/metadata-acceptance-'+[guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $evidence|Out-Null
$copy=Join-Path $evidence 'metadata snapshot.dump'
Copy-Item -LiteralPath $MetadataArchive -Destination $copy
$originalHash=(Get-FileHash -LiteralPath $MetadataArchive).Hash
$executable=(Get-Process -Id $PID).Path
$command=Join-Path $PSScriptRoot 'metadata-check.ps1'
$records=[Collections.Generic.List[object]]::new()
function Check($Archive,$Expected,$Code,$Report=''){
 $arguments=@('-NoProfile','-NonInteractive','-File',$command,'-Archive',$Archive)
 if($Report){$arguments+=@('-ReportPath',$Report)}
 $result=Invoke-RestivraNative -FilePath $executable -Arguments $arguments -TimeoutSeconds 90
 $reportData=$result.Output|ConvertFrom-Json
 if($reportData.status -ne $Expected -or $reportData.error_code -ne $Code -or $reportData.worker_started -or $reportData.cleanup -notin @('passed','not_needed')){throw "Metadata case expected $Expected/$Code; got $($reportData.status)/$($reportData.error_code), cleanup=$($reportData.cleanup)"}
 if(($Expected -eq 'passed' -and $result.ExitCode -ne 0) -or ($Expected -eq 'failed' -and $result.ExitCode -eq 0)){throw 'Metadata command exit code did not match its report'}
 if($reportData.cleanup -eq 'passed'){
  $left=Invoke-RestivraNative -FilePath docker -Arguments @('ps','--all','--quiet','--filter',"name=^/$($reportData.temporary_container)$")
  if($left.ExitCode -ne 0 -or $left.Output.Trim()){throw 'Metadata checker left its temporary database'}
 }
 $records.Add($reportData)
 return $reportData
}
$reportPath=Join-Path $evidence 'valid report.json'
$valid=Check $copy 'passed' '' $reportPath
if($valid.restore -ne 'passed' -or $valid.schema -ne 'passed' -or -not $valid.counts.valid_installation){throw 'Valid metadata restore was incomplete'}
$reportHash=(Get-FileHash -LiteralPath $reportPath).Hash
Check $copy 'failed' 'report_path_unavailable' $reportPath|Out-Null
if((Get-FileHash -LiteralPath $reportPath).Hash -ne $reportHash){throw 'Existing metadata report was overwritten'}
Check (Resolve-Path -LiteralPath $ApplicationArchive).Path 'failed' 'metadata_schema_invalid'|Out-Null
$corrupt=Join-Path $evidence 'corrupt.dump'
[IO.File]::WriteAllBytes($corrupt,[Text.Encoding]::ASCII.GetBytes('PGDMPcorrupt'))
Check $corrupt 'failed' 'metadata_restore_failed'|Out-Null
$plain=Join-Path $evidence 'plain.txt'
[IO.File]::WriteAllText($plain,'not an archive')
Check $plain 'failed' 'archive_invalid'|Out-Null
$priorHost=$env:DOCKER_HOST
try {$env:DOCKER_HOST='tcp://do-not-connect.invalid:2375';Check $copy 'failed' 'engine_not_local'|Out-Null}
finally {$env:DOCKER_HOST=$priorHost}
$contextName='restivra-metadata-context-'+[guid]::NewGuid().ToString('N')
$priorContext=$env:DOCKER_CONTEXT
$localEndpoint=Invoke-RestivraNative -FilePath docker -Arguments @('context','inspect','--format','{{.Endpoints.docker.Host}}')
$contextCreated=$false
try {
 $created=Invoke-RestivraNative -FilePath docker -Arguments @('context','create',$contextName,'--docker','host=tcp://do-not-connect.invalid:2375')
 if($created.ExitCode -ne 0){throw 'Could not create isolated context fixture'}
 $contextCreated=$true
 $env:DOCKER_CONTEXT=$contextName
 $env:DOCKER_HOST=$localEndpoint.Output.Trim()
 Check $copy 'failed' 'engine_not_local'|Out-Null
} finally {
 $env:DOCKER_CONTEXT=$priorContext
 $env:DOCKER_HOST=$priorHost
 if($contextCreated){$removed=Invoke-RestivraNative -FilePath docker -Arguments @('context','rm','--force',$contextName);if($removed.ExitCode -ne 0){throw 'Could not remove isolated context fixture'}}
}
$isolatedConfig=Join-Path $evidence 'isolated-docker-config'
New-Item -ItemType Directory -Path $isolatedConfig|Out-Null
$isolatedName='restivra-isolated-remote-'+[guid]::NewGuid().ToString('N')
$created=Invoke-RestivraNative -FilePath docker -Arguments @('--config',$isolatedConfig,'context','create',$isolatedName,'--docker','host=tcp://do-not-connect.invalid:2375')
if($created.ExitCode -ne 0){throw 'Could not create persistent-context fixture'}
[IO.File]::WriteAllText((Join-Path $isolatedConfig 'config.json'),(@{currentContext=$isolatedName}|ConvertTo-Json))
$priorConfig=$env:DOCKER_CONFIG
try {
 $env:DOCKER_CONFIG=$isolatedConfig
 $env:DOCKER_CONTEXT=$null
 $env:DOCKER_HOST=$null
 Check $copy 'failed' 'engine_not_local'|Out-Null
} finally {$env:DOCKER_CONFIG=$priorConfig;$env:DOCKER_CONTEXT=$priorContext;$env:DOCKER_HOST=$priorHost}
if((Get-FileHash -LiteralPath $MetadataArchive).Hash -ne $originalHash){throw 'Original metadata archive changed'}
$records|ConvertTo-Json -Depth 8|Set-Content (Join-Path $evidence 'results.json')
@{passed=$true;cases=$records.Count;original_archive_unchanged=$true;existing_report_preserved=$true;remote_context_rejected=$true;worker_never_started=$true;verified_at=[datetime]::UtcNow.ToString('o')}|ConvertTo-Json|Set-Content (Join-Path $evidence 'summary.json')
Write-Host "Metadata acceptance passed: $($records.Count) cases. Evidence: $evidence"
