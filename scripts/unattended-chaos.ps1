#Requires -Version 7.2
param([Parameter(Mandatory)][string]$ProjectDirectory,[string]$Base='http://127.0.0.1:19474',[string]$EvidenceDirectory='work/unattended-chaos')
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'api-session.ps1')
# This destructive fault test targets only the dedicated disposable stack.
# Reject the regular installation or an unrelated container with the same name.
$testRoot=(Resolve-Path -LiteralPath $ProjectDirectory).Path
$db=docker inspect restivra-pilot-chaos-db-1|ConvertFrom-Json
if($LASTEXITCODE -ne 0 -or $db.Config.Labels.'com.docker.compose.project' -ne 'restivra-pilot-chaos'){throw 'Expected the isolated restivra-pilot-chaos database'}
if($db.Config.Labels.'com.docker.compose.project.working_dir'.Replace('/','\') -ne $testRoot.Replace('/','\')){throw 'ProjectDirectory does not own the isolated database'}
$worker=docker inspect restivra-pilot-chaos-worker-1|ConvertFrom-Json
if($LASTEXITCODE -ne 0 -or $worker.Config.Labels.'com.docker.compose.project' -ne 'restivra-pilot-chaos' -or 'DOCKER_HOST=tcp://fault-proxy:2375' -notin $worker.Config.Env){throw 'Expected a fault-proxy worker in the isolated stack'}
$proxy=docker inspect restivra-fault-proxy|ConvertFrom-Json
if($LASTEXITCODE -ne 0 -or @($proxy.NetworkSettings.Networks.PSObject.Properties.Name) -notcontains 'restivra-pilot-chaos_control' -or $proxy.HostConfig.PortBindings.PSObject.Properties.Count -gt 0){throw 'Expected an unpublished fault proxy on the isolated control network'}
New-Item -ItemType Directory -Path $EvidenceDirectory -Force|Out-Null
$c=New-RestivraSession -Base $Base -EnvFile (Join-Path $ProjectDirectory '.env')
function API([string]$Path,[string]$Method='Get',$Body=$null){$a=@{Uri="$Base/api$Path";Method=$Method;WebSession=$c.WebSession;Headers=$c.Headers;TimeoutSec=15;MaximumRedirection=0};if($Method -ne 'Get'){$a.ContentType='application/json';$a.Body=$Body|ConvertTo-Json -Depth 10};Invoke-RestMethod @a}
function New-ChaosProject([string]$Name,[int]$Timeout,[string]$SQL){$cfg=$c.Session.default_config|ConvertTo-Json -Depth 8|ConvertFrom-Json;$cfg.timeout_seconds=$Timeout;$cfg.checks=@(@{name=$Name;sql=$SQL});API '/projects' 'Post' @{name=$Name;backup_pattern='commerce.dump';config=$cfg;schedule_time='';timezone='UTC'}}
function Wait-Final([string]$ID){$limit=[DateTime]::UtcNow.AddMinutes(5);do{try{$job=API "/jobs/$ID";if($job.status -notin @('queued','running')){return $job}}catch{};Start-Sleep -Milliseconds 500}while([DateTime]::UtcNow -lt $limit);throw 'Recovery exceeded five-minute deadline'}
function Assert-Recovered($Job,[string]$Scenario){
 if($Job.status -ne 'passed' -or $Job.attempt_count -lt 2 -or $Job.attempts.Count -lt 2 -or $Job.report.cleanup_status -ne 'passed'){throw "$Scenario did not recover with attempt evidence"}
 if($Job.attempts[0].status -notin @('failed','interrupted')){throw 'Prior failure was lost'}
 if(docker ps -aq --filter "label=restivra.run=$($Job.id)"){throw 'Recovery leaked a restore container'}
 $Job|ConvertTo-Json -Depth 20|Set-Content -LiteralPath (Join-Path $EvidenceDirectory "$Scenario.json")
 Write-Output "$Scenario recovered after $($Job.attempt_count) attempts; cleanup passed."
}
$first=New-ChaosProject 'Chaos · Docker unresponsive 35 seconds' 10 'SELECT count(*) = 100000 FROM public.order_items'
docker exec restivra-fault-proxy /fault-proxy arm
if($LASTEXITCODE -ne 0){throw 'Could not arm the isolated Docker proxy'}
$job=API '/jobs' 'Post' @{project_id=$first.id}
Assert-Recovered (Wait-Final $job.id) 'docker-stall'
$second=New-ChaosProject 'Chaos · metadata unavailable 25 seconds' 180 'SELECT true FROM (SELECT pg_sleep(8)) AS waited'
$job=API '/jobs' 'Post' @{project_id=$second.id}
for($i=0;$i -lt 120;$i++){$state=API "/jobs/$($job.id)";if($state.status -eq 'running'){break};Start-Sleep -Milliseconds 200}
if($state.status -ne 'running'){throw 'Metadata stall target did not start'}
docker pause restivra-pilot-chaos-db-1|Out-Null
if($LASTEXITCODE -ne 0){throw 'Could not pause isolated metadata database'}
try{Start-Sleep -Seconds 25}finally{docker unpause restivra-pilot-chaos-db-1|Out-Null}
Assert-Recovered (Wait-Final $job.id) 'metadata-stall'
$third=New-ChaosProject 'Chaos · cancel scheduled retry' 10 'SELECT true'
docker exec restivra-fault-proxy /fault-proxy arm|Out-Null
if($LASTEXITCODE -ne 0){throw 'Could not arm cancellation fault'}
$job=API '/jobs' 'Post' @{project_id=$third.id}
$limit=[DateTime]::UtcNow.AddMinutes(2)
do{$state=API "/jobs/$($job.id)";if($state.status -eq 'queued' -and $state.attempt_count -eq 1 -and $state.retry_at){break};Start-Sleep -Milliseconds 300}while([DateTime]::UtcNow -lt $limit)
if($state.status -ne 'queued' -or -not $state.retry_at){throw 'No delayed retry to cancel'}
API "/jobs/$($job.id)/cancel" 'Post'|Out-Null
# Observe beyond the persisted 30-second deadline, while cleanup reconciles.
Start-Sleep -Seconds 35
$state=API "/jobs/$($job.id)"
if($state.status -ne 'cancelled' -or $state.retry_at -or $state.error -ne 'Cancelled by user.' -or $state.attempt_count -ne 1 -or $state.attempts.Count -ne 1){throw 'Cancelled delayed retry ran again, retained retry metadata or lost its evidence'}
if(docker ps -aq --filter "label=restivra.run=$($job.id)"){throw 'Cancelled retry left a restore container'}
$state|ConvertTo-Json -Depth 20|Set-Content (Join-Path $EvidenceDirectory 'cancelled-retry.json')
Write-Output 'Delayed retry remained cancelled after its original deadline; cleanup passed.'
@{passed=$true;docker_stall_seconds=35;metadata_stall_seconds=25;cancelled_retry_suppressed=$true;manual_retry_required=$false;finished_at=[DateTime]::UtcNow.ToString('o')}|ConvertTo-Json|Set-Content (Join-Path $EvidenceDirectory 'summary.json')
