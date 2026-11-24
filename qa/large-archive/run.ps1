#Requires -Version 7.2
# Fresh synthetic installation only. Existing fixtures require an explicit hash.
[CmdletBinding(DefaultParameterSetName='Generate')]
param(
    [Parameter(Mandatory)][string]$ApplicationImage,
    [Parameter(Mandatory,ParameterSetName='Existing')][string]$Archive,
    [Parameter(Mandatory,ParameterSetName='Existing')][ValidatePattern('^[a-f0-9]{64}$')][string]$ExpectedSHA256,
    [ValidateRange(1,30)][int]$Repeats=8,
    [ValidateRange(1024,65535)][int]$Port=27485
)
$ErrorActionPreference='Stop'
$sourceRoot=Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
. (Join-Path $sourceRoot 'scripts/release-ops.ps1')
. (Join-Path $sourceRoot 'scripts/qa-environment.ps1')
Release-LocalEngine
$imageID=Release-ImageID $ApplicationImage
if($Archive){
    $Archive=(Resolve-Path -LiteralPath $Archive).Path
    $length=(Get-Item -LiteralPath $Archive).Length
    if($length -lt 32MB -or $length -gt 256MB -or (Get-FileHash -LiteralPath $Archive).Hash.ToLowerInvariant() -cne $ExpectedSHA256){throw 'Existing synthetic fixture size/hash does not match the reviewed reproduction archive.'}
}
$suffix=[guid]::NewGuid().ToString('N')
$root=Join-Path $sourceRoot "work/large-archive-$suffix"
$null=New-Item -ItemType Directory $root
Copy-Item (Join-Path $sourceRoot 'compose.yaml') $root
Copy-Item (Join-Path $sourceRoot 'scripts') (Join-Path $root 'scripts') -Recurse
$project="restivra-large-$($suffix.Substring(0,12))"
$qaEnvironment=Enter-RestivraQAEnvironment $project $Port
$script:ReleaseRoot=$root
$source="restivra-large-source-$suffix";$sourceCreated=$false
$records=[Collections.Generic.List[object]]::new()
$proof=[ordered]@{status='failed';synthetic_data=$true;fixture_mode=$PSCmdlet.ParameterSetName;application_image=$imageID;started_at=[datetime]::UtcNow.ToString('o')}
function API([string]$Path,[string]$Method='Get',$Body=$null){
    $options=@{Uri="$base/api$Path";Method=$Method;WebSession=$connection.WebSession;Headers=$connection.Headers;TimeoutSec=15;MaximumRedirection=0}
    if($Method -ne 'Get'){$options.ContentType='application/json';$options.Body=if($null -eq $Body){'{}'}else{$Body|ConvertTo-Json -Depth 12 -Compress}}
    Invoke-RestMethod @options
}
function New-LargeProject([string]$Name,[string]$Pattern='capacity.dump',[object[]]$Checks=$capacityChecks,[int]$DataMB=1024){
    $cfg=$connection.Session.default_config|ConvertTo-Json -Depth 8|ConvertFrom-Json
    $cfg.data_mb=$DataMB;$cfg.memory_mb=1536;$cfg.checks=$Checks
    API '/projects' 'Post' @{name=$Name;backup_pattern=$Pattern;config=$cfg;schedule_time='';timezone='UTC'}
}
function Wait-LargeJob([string]$ID){
    $deadline=[datetime]::UtcNow.AddMinutes(6)
    do{$job=API "/jobs/$ID";if($job.status -notin @('queued','running')){return $job};Start-Sleep -Milliseconds 400}while([datetime]::UtcNow -lt $deadline)
    throw 'Large-archive job exceeded the test deadline.'
}
function Assert-LargeJob($Job,[string]$Expected,[string]$Scenario){
    if($Job.status -cne $Expected){throw "Unexpected large-archive outcome: $Scenario."}
    if($Job.report -and $Job.report.cleanup_status -notin @('passed','not_needed')){throw 'Large-archive cleanup did not succeed.'}
    if(Release-Docker @('ps','--all','--quiet','--filter',"label=restivra.run=$($Job.id)")){throw 'A large-archive restore container remains.'}
    if($Expected -eq 'passed'){
        if($Job.report.restore_status -ne 'passed' -or $Job.report.checks.Count -ne 3 -or @($Job.report.checks|Where-Object{$_.status -ne 'passed'}).Count -or $Job.report.backup.sha256 -cne $proof.archive_sha256){throw 'Passing large-archive result lacks the expected restore/checks/content evidence.'}
    }
    if($Scenario -eq 'false_check' -and ($Job.report.restore_status -ne 'passed' -or $Job.report.checks.Count -ne 1 -or $Job.report.checks[0].status -ne 'failed')){throw 'False-check case did not reach its intended SQL failure.'}
    if($Scenario -in @('truncated_data','data_capacity') -and $Job.report.restore_status -ne 'failed'){throw 'Restore failure fixture did not fail in the restore stage.'}
    if($Job.report.application_build){
        if($Job.report.application_build.revision -cne $runtime.revision -or $Job.report.application_build.version -cne $runtime.version){throw 'Large-archive report records a different verifier build.'}
    }elseif($Job.report){throw 'Large-archive report lacks verifier provenance.'}
    $record=[ordered]@{scenario=$Scenario;id=$Job.id;status=$Job.status;cleanup=$Job.report.cleanup_status;attempts=$Job.attempt_count;error_code=$Job.report.error_code;duration_ms=$Job.report.duration_ms}
    $records.Add($record)
    $record|ConvertTo-Json -Compress|Add-Content (Join-Path $root 'runs.ndjson')
    $Job|ConvertTo-Json -Depth 20|Set-Content (Join-Path $root "$($Job.id).json")
    Write-Host "$Scenario : $Expected"
}
function Run-Large($Project,[string]$Expected,[string]$Scenario){
    $job=API '/jobs' 'Post' @{project_id=$Project.id}
    $done=Wait-LargeJob $job.id
    Assert-LargeJob $done $Expected $Scenario
}
function Wait-RestoreProcess([string]$JobID){
    $deadline=[datetime]::UtcNow.AddSeconds(45)
    do{
        $ids=Release-Docker @('ps','--quiet','--filter',"label=restivra.run=$JobID")
        if($ids){
            $processes=Release-Docker @('top',$ids,'-eo','pid,args')
            if($processes -match 'pg_restore --username'){return $ids}
        }
        if((API "/jobs/$JobID").status -notin @('queued','running')){throw 'Restore finished before the intended fault observation.'}
        Start-Sleep -Milliseconds 100
    }while([datetime]::UtcNow -lt $deadline)
    throw 'No actual pg_restore process was observed for fault injection.'
}
try{
    $runtime=(Release-Docker @('run','--rm','--network','none','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges:true',$imageID,'version'))|ConvertFrom-Json
    $proof.build=$runtime
    & (Join-Path $root 'scripts/setup.ps1')
    Release-SetImage $imageID
    $copy=Join-Path $root 'backups/capacity.dump'
    if($Archive){Copy-Item -LiteralPath $Archive -Destination $copy}
    else{
        Release-Docker @('run','--detach','--name',$source,'--network','none','--memory','768m','--cpus','1','--pids-limit','128','--env','POSTGRES_HOST_AUTH_METHOD=trust','postgres:17-bookworm')|Out-Null
        $sourceCreated=$true;$ready=$false
        for($n=0;$n -lt 120;$n++){
            try{Release-Docker @('exec',$source,'sh','-c','test "$(cat /proc/1/comm)" = postgres && pg_isready -q -U postgres')|Out-Null;$ready=$true;break}catch{Start-Sleep -Milliseconds 500}
        }
        if(-not $ready){throw 'Disposable capacity source did not become ready.'}
        Release-Docker @('cp',(Join-Path $sourceRoot 'testdata/capacity.sql'),"${source}:/tmp/capacity.sql")|Out-Null
        Release-Docker @('exec',$source,'psql','-X','-U','postgres','-v','ON_ERROR_STOP=1','-f','/tmp/capacity.sql') 180|Out-Null
        Release-Docker @('exec',$source,'pg_dump','-U','postgres','-Fc','-f','/tmp/capacity.dump','postgres') 180|Out-Null
        Release-Docker @('cp',"${source}:/tmp/capacity.dump",$copy)|Out-Null
        Release-Docker @('rm','--force','--volumes',$source)|Out-Null;$sourceCreated=$false
    }
    $proof.archive_bytes=(Get-Item $copy).Length
    $proof.archive_sha256=(Get-FileHash $copy).Hash.ToLowerInvariant()
    if($proof.archive_bytes -lt 32MB -or ($Archive -and $proof.archive_sha256 -cne $ExpectedSHA256)){throw 'Copied capacity fixture does not meet the reproduction size/hash.'}
    $truncated=Join-Path $root 'backups/truncated.dump'
    Copy-Item -LiteralPath $copy -Destination $truncated
    $stream=[IO.File]::Open($truncated,[IO.FileMode]::Open,[IO.FileAccess]::Write)
    try{$stream.SetLength($proof.archive_bytes-1MB)}finally{$stream.Dispose()}
    $invalid=[byte[]]::new(8MB+12)
    [Text.Encoding]::ASCII.GetBytes('PGDMPinvalid!').CopyTo($invalid,0)
    [IO.File]::WriteAllBytes((Join-Path $root 'backups/invalid.dump'),$invalid)
    Release-Docker @('compose','up','--no-build','--pull','never','-d','--wait','--wait-timeout','120') 150|Out-Null
    . (Join-Path $root 'scripts/api-session.ps1')
    $base="http://127.0.0.1:$Port"
    $connection=New-RestivraSession -Base $base -EnvFile (Join-Path $root '.env')
    $capacityChecks=@(
        @{name='All payload rows';sql='SELECT count(*) = 100000 FROM public.payloads'},
        @{name='Payload width';sql='SELECT min(length(payload)) = 1024 AND max(length(payload)) = 1024 FROM public.payloads'},
        @{name='Expanded text';sql='SELECT count(*) = 128 AND min(length(payload)) = 1120000 FROM public.expansion_probe'}
    )
    $good=New-LargeProject 'Synthetic large archive'
    $schedule=New-LargeProject 'Synthetic large archive schedule'
    $schedule.schedule_time=[datetime]::UtcNow.AddMinutes(2).ToString('HH:mm')
    $schedule=API "/projects/$($schedule.id)" 'Put' $schedule
    for($n=1;$n -le $Repeats;$n++){Run-Large $good 'passed' "repeat_$n"}
    Run-Large (New-LargeProject 'Invalid large header' 'invalid.dump') 'failed' 'invalid_header'
    Run-Large (New-LargeProject 'Truncated large archive' 'truncated.dump') 'failed' 'truncated_data'
    Run-Large (New-LargeProject 'Deliberately false check' 'capacity.dump' @(@{name='False rule';sql='SELECT false'})) 'failed' 'false_check'
    Run-Large (New-LargeProject 'Insufficient restored-data capacity' 'capacity.dump' $capacityChecks 128) 'failed' 'data_capacity'
    $cancel=New-LargeProject 'Cancel real large restore'
    $job=API '/jobs' 'Post' @{project_id=$cancel.id}
    $null=Wait-RestoreProcess $job.id
    API "/jobs/$($job.id)/cancel" 'Post'|Out-Null
    Assert-LargeJob (Wait-LargeJob $job.id) 'cancelled' 'cancel_during_real_restore'
    $job=API '/jobs' 'Post' @{project_id=$good.id}
    $null=Wait-RestoreProcess $job.id
    $worker=Release-Service worker
    Release-Docker @('kill','--signal','SIGKILL',$worker.id)|Out-Null
    Release-Docker @('start',$worker.id)|Out-Null
    $recovered=Wait-LargeJob $job.id
    Assert-LargeJob $recovered 'passed' 'actual_worker_sigkill_during_restore'
    if($recovered.attempt_count -lt 2 -or $recovered.attempts[0].status -ne 'interrupted'){throw 'Worker fault did not preserve its interrupted first attempt.'}
    Run-Large $good 'passed' 'restore_after_fault'
    $deadline=[datetime]::UtcNow.AddMinutes(3)
    do{
        $history=API "/jobs?project_id=$($schedule.id)"
        $scheduled=@($history|Where-Object trigger -eq 'schedule')
        if($scheduled.Count){break}
        Start-Sleep -Seconds 1
    }while([datetime]::UtcNow -lt $deadline)
    if($scheduled.Count -ne 1){throw 'Expected one actual wall-clock scheduled restore.'}
    Assert-LargeJob (Wait-LargeJob $scheduled[0].id) 'passed' 'wall_clock_schedule'
    $schedule.schedule_time='';API "/projects/$($schedule.id)" 'Put' $schedule|Out-Null
    foreach($record in $records){if((API "/jobs/$($record.id)").status -cne $record.status){throw 'Retained large-archive outcome changed.'}}
    foreach($service in @('api','worker')){if((Release-Service $service).image -cne $imageID){throw 'Application image changed during large-archive acceptance.'}}
    $installation=Release-SQL 'SELECT id FROM installation'
    if(Release-Docker @('ps','--all','--quiet','--filter',"label=restivra.installation=$installation")){throw 'An owned large-archive restore container remains.'}
    if((Release-Docker @('compose','exec','-T','worker','sh','-c','find /work/jobs -type f | wc -l')).Trim() -ne '0'){throw 'Large-archive private snapshots remain.'}
    if((Get-FileHash $copy).Hash.ToLowerInvariant() -cne $proof.archive_sha256 -or ($Archive -and (Get-FileHash $Archive).Hash.ToLowerInvariant() -cne $ExpectedSHA256)){throw 'The source/copy archive changed.'}
    $proof.persisted_outcomes_rechecked=$records.Count;$proof.restore_containers=0;$proof.private_job_files=0;$proof.status='passed'
}finally{
    if($schedule){try{$schedule.schedule_time='';API "/projects/$($schedule.id)" 'Put' $schedule|Out-Null}catch{}}
    if($sourceCreated){try{Release-Docker @('rm','--force','--volumes',$source)|Out-Null}catch{Write-Warning 'Capacity source cleanup needs review.'}}
    $proof.finished_at=[datetime]::UtcNow.ToString('o');$proof.asserted_runs=$records.Count
    Release-JSON (Join-Path $root 'summary.json') $proof
    try{$null=Release-Service db -AllowMissing;Release-Docker @('compose','down') 90|Out-Null}catch{Write-Warning 'Large-archive installation cleanup needs review.'}
    $script:ReleaseRoot=$sourceRoot;Restore-RestivraQAEnvironment $qaEnvironment
    Write-Host "Large-archive evidence and private configuration retained under $root"
}
