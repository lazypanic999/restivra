#Requires -Version 7.2
param(
    [Parameter(Mandatory)][string]$ProjectDirectory,
    [string]$ComposeProject = 'restivra-pilot-s3-acceptance',
    [string]$Base = 'http://127.0.0.1:28474',
    [int]$Minutes = 235,
    [string]$EvidenceDirectory = 'work/s3-soak',
    [string]$SourceProfile = 'qa-storage',
    [string]$Bucket = 'restivra-synthetic',
    [Parameter(Mandatory)][string]$ExpectedSHA256
)
# Fault injection is restricted to a disposable Compose stack with a service
# named storage. It must contain synthetic data, never a real provider account.
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'api-session.ps1')
if ($ComposeProject -notmatch '^restivra-pilot-[a-z0-9-]+$') { throw 'Expected an isolated restivra-pilot-* project.' }
if ($Minutes -lt 3 -or $Minutes -gt 300) { throw 'Minutes must be 3–300.' }
if ($ExpectedSHA256 -notmatch '^[a-f0-9]{64}$') { throw 'Expected a synthetic fixture SHA-256.' }
$ProjectDirectory = (Resolve-Path -LiteralPath $ProjectDirectory).Path
New-Item -ItemType Directory -Path $EvidenceDirectory -Force | Out-Null
$evidence = (Resolve-Path -LiteralPath $EvidenceDirectory).Path
if (Test-Path (Join-Path $evidence 'runs.ndjson')) { throw 'Use a new evidence directory for each run.' }
$connection = New-RestivraSession -Base $Base -EnvFile (Join-Path $ProjectDirectory '.env')
function API([string]$Path,[string]$Method='Get',$Body=$null) {
    $options=@{Uri="$Base/api$Path";Method=$Method;WebSession=$connection.WebSession;Headers=$connection.Headers;TimeoutSec=15;MaximumRedirection=0}
    if($Method -ne 'Get'){$options.ContentType='application/json';$options.Body=if($null -eq $Body){'{}'}else{$Body|ConvertTo-Json -Depth 12 -Compress}}
    Invoke-RestMethod @options
}
function DC([string[]]$Arguments) {
    $result=& docker compose --project-directory $ProjectDirectory -p $ComposeProject @Arguments
    if($LASTEXITCODE -ne 0){throw "Isolated Compose command failed: $($Arguments[0])"}
    return $result
}
function Runtime {
    $rows=@()
    foreach($service in @('api','worker','db','storage')) {
        $id=DC @('ps','--all','--quiet',$service)
        if(-not $id){throw "Missing isolated service: $service"}
        $item=@(docker inspect $id|ConvertFrom-Json)[0]
        if($LASTEXITCODE -ne 0 -or $item.Config.Labels.'com.docker.compose.project' -ne $ComposeProject){throw 'Unexpected Compose ownership'}
        $rows+=@{service=$service;container=$item.Id;image=$item.Image;restarts=$item.RestartCount}
    }
    return $rows
}
$runtime=Runtime
$storage=@(docker inspect (DC @('ps','--quiet','storage'))|ConvertFrom-Json)[0]
if($storage.Config.Image -notmatch '^rustfs/rustfs:' -or -not @($storage.Mounts|Where-Object {$_.Type -eq 'volume' -and $_.Destination -eq '/data'}).Count){throw 'Fault test requires disposable RustFS with persistent /data volume.'}
$worker=@(docker inspect (DC @('ps','--quiet','worker'))|ConvertFrom-Json)[0]
$instance=@($worker.Config.Env|Where-Object {$_ -like 'RESTIVRA_INSTANCE=*'})[0].Substring('RESTIVRA_INSTANCE='.Length)
if(-not $instance -or $instance -eq 'restivra-main'){throw 'Refusing faults against the main instance'}
if(@((API '/jobs')|Where-Object {$_.status -in @('queued','running')}).Count){throw 'Start with an idle disposable stack.'}
$runtime|ConvertTo-Json -Depth 5|Set-Content (Join-Path $evidence 'runtime-start.json')
$started=[datetime]::UtcNow
$deadline=$started.AddMinutes($Minutes)
$records=[Collections.Generic.List[object]]::new()
$projects=[Collections.Generic.List[object]]::new()
function Project($Name,$Pattern,$SQL) {
    $config=$connection.Session.default_config|ConvertTo-Json -Depth 10|ConvertFrom-Json
    $config.checks=@(@{name=$Name;sql=$SQL})
    $p=API '/projects' 'Post' @{name=$Name;backup_pattern=$Pattern;source=@{kind='s3';profile=$SourceProfile;bucket=$Bucket};config=$config;schedule_time='';timezone='UTC'}
    $projects.Add($p)
    return $p
}
function Wait-Job($ID) {
    $limit=[datetime]::UtcNow.AddMinutes(4)
    do {$j=API "/jobs/$ID";if($j.status -notin @('queued','running')){return $j};Start-Sleep -Milliseconds 400}while([datetime]::UtcNow -lt $limit)
    throw 'S3 job exceeded test deadline'
}
function Assert-Job($Job,$Expected,$Scenario) {
    if($Job.status -ne $Expected){throw "$Scenario expected $Expected, got $($Job.status)"}
    if($Job.report -and $Job.report.cleanup_status -notin @('passed','not_needed')){throw "$Scenario failed cleanup"}
    if($Expected -eq 'passed' -and $Job.report.backup.sha256 -ne $ExpectedSHA256){throw "$Scenario verified different bytes"}
    if(docker ps --all --quiet --filter "label=restivra.run=$($Job.id)"){throw "$Scenario leaked a container"}
    if($LASTEXITCODE -ne 0){throw 'Docker container check failed'}
    $row=[ordered]@{scenario=$Scenario;id=$Job.id;project_id=$Job.project_id;status=$Job.status;error_code=$Job.report.error_code;cleanup=$Job.report.cleanup_status;sha256=$Job.report.backup.sha256;source=$Job.report.backup.source;attempt_count=$Job.attempt_count;attempts=@($Job.attempts|ForEach-Object {@{number=$_.number;status=$_.status;error_code=$_.report.error_code;has_report=($null -ne $_.report)}});finished_at=$Job.finished_at}
    $records.Add($row)
    $row|ConvertTo-Json -Compress -Depth 8|Add-Content (Join-Path $evidence 'runs.ndjson')
}
function Run($Project,$Expected,$Scenario) { $j=API '/jobs' 'Post' @{project_id=$Project.id};Assert-Job (Wait-Job $j.id) $Expected $Scenario }
function Wait-Restore($Job) {
    $limit=[datetime]::UtcNow.AddSeconds(40)
    do {
        $j=API "/jobs/$($Job.id)"
        if($j.status -eq 'running' -and (docker ps --quiet --filter "label=restivra.run=$($Job.id)")){return}
        Start-Sleep -Milliseconds 150
    }while([datetime]::UtcNow -lt $limit)
    throw 'Could not observe a real running restore container'
}
$schedule=$null
$failure=$null
$iteration=0
try {
    $good=Project 'Soak / S3 rows' 'daily/*.dump' 'SELECT count(*)=2 FROM public.customers'
    $falseProject=Project 'Soak / S3 false' 'daily/*.dump' 'SELECT false'
    $missing=Project 'Soak / S3 missing' 'missing/*.dump' 'SELECT true'
    $bad=Project 'Soak / S3 corrupt' 'corrupt/*.dump' 'SELECT true'
    $slow=Project 'Soak / S3 interruption' 'daily/*.dump' 'SELECT true FROM (SELECT pg_sleep(8)) AS waited'
    $schedule=Project 'Soak / S3 schedule' 'daily/*.dump' 'SELECT count(*)=2 FROM public.customers'
    $schedule.schedule_time=[datetime]::UtcNow.AddMinutes(2).ToString('HH:mm')
    $schedule=API "/projects/$($schedule.id)" 'Put' $schedule
    do {
        $iteration++
        Run $good 'passed' 'repeat_s3_restore'
        if($iteration % 20 -eq 1){Run $falseProject 'failed' 'false_check';Run $missing 'failed' 'missing_object';Run $bad 'failed' 'corrupt_object'}
        if($iteration -in @(2,60,120)) {
            DC @('stop','--timeout','5','storage')|Out-Null
            try {
                $j=API '/jobs' 'Post' @{project_id=$good.id}
                $limit=[datetime]::UtcNow.AddSeconds(60)
                do{$retry=API "/jobs/$($j.id)";if($retry.retry_at){break};Start-Sleep -Milliseconds 300}while([datetime]::UtcNow -lt $limit)
                if(-not $retry.retry_at -or $retry.attempts[0].report.error_code -ne 'source_unavailable'){throw 'Storage outage did not preserve a retryable source failure'}
            } finally {DC @('start','storage')|Out-Null}
            $recovered=Wait-Job $j.id
            Assert-Job $recovered 'passed' 's3_outage_recovered'
            if($recovered.attempt_count -lt 2 -or -not $recovered.report.backup.source.version_id){throw 'Storage recovery lost version or attempt evidence'}
        }
        if($iteration -in @(3,90,180)) {
            $j=API '/jobs' 'Post' @{project_id=$slow.id}
            Wait-Restore $j
            DC @('kill','--signal','SIGKILL','worker')|Out-Null
            DC @('start','worker')|Out-Null
            $recovered=Wait-Job $j.id
            Assert-Job $recovered 'passed' 'worker_sigkill_recovered'
            if($recovered.attempt_count -lt 2 -or $recovered.attempts[0].status -ne 'interrupted'){throw 'Worker interruption evidence missing'}
        }
        if($iteration -in @(5,100,200)) {
            DC @('restart','db')|Out-Null
            $ready=$false
            for($n=0;$n -lt 60;$n++){try{if((API '/health').worker_online){$ready=$true;break}}catch{};Start-Sleep -Seconds 1}
            if(-not $ready){throw 'Worker did not recover after metadata database restart'}
            Run $good 'passed' 'metadata_database_restart'
        }
        if($iteration % 15 -eq 0) {
            $j=API '/jobs' 'Post' @{project_id=$slow.id};Wait-Restore $j
            API "/jobs/$($j.id)/cancel" 'Post'|Out-Null
            Assert-Job (Wait-Job $j.id) 'cancelled' 'running_restore_cancelled'
        }
        if($iteration % 10 -eq 0) {
            $sample=DC @('exec','-T','worker','sh','-c','find /work/jobs -type f | wc -l')
            if([int]$sample -ne 0){throw 'Private snapshot leaked between jobs'}
            @{at=[datetime]::UtcNow.ToString('o');private_files=[int]$sample;containers=@(docker ps --all --quiet --filter "label=restivra.instance=$instance").Count;stats=@(docker stats --no-stream --format '{{json .}}' (DC @('ps','--quiet')))|ForEach-Object {$_|ConvertFrom-Json}}|ConvertTo-Json -Depth 5 -Compress|Add-Content (Join-Path $evidence 'resources.ndjson')
        }
        Write-Host "S3 soak: iteration $iteration, $($records.Count) verified outcomes."
        $remaining=($deadline-[datetime]::UtcNow).TotalSeconds
        if($remaining -gt 0){Start-Sleep -Seconds ([Math]::Min(45,[Math]::Ceiling($remaining)))}
    }while([datetime]::UtcNow -lt $deadline)
    $scheduled=@((API "/jobs?project_id=$($schedule.id)")|Where-Object trigger -eq 'schedule')
    if($scheduled.Count -ne 1){throw 'Expected exactly one wall-clock scheduled S3 run'}
    foreach($j in $scheduled){Assert-Job (Wait-Job $j.id) 'passed' 'wall_clock_s3_schedule'}
    $final=Runtime
    foreach($initial in $runtime){$last=@($final|Where-Object service -eq $initial.service)[0];if($last.container -ne $initial.container -or $last.image -ne $initial.image){throw 'A service was recreated or its image changed during the soak'}}
    $final|ConvertTo-Json -Depth 5|Set-Content (Join-Path $evidence 'runtime-finish.json')
} catch {$failure=$_.Exception.Message;throw}
finally {
    if($schedule){try{$schedule.schedule_time='';API "/projects/$($schedule.id)" 'Put' $schedule|Out-Null}catch{}}
    $finished=[datetime]::UtcNow
    [ordered]@{started_at=$started.ToString('o');finished_at=$finished.ToString('o');elapsed_minutes=[Math]::Round(($finished-$started).TotalMinutes,2);iterations=$iteration;runs=$records.Count;failure=$failure;fixture_sha256=$ExpectedSHA256;outcomes=@($records|Group-Object status|ForEach-Object {@{status=$_.Name;count=$_.Count}})}|ConvertTo-Json -Depth 5|Set-Content (Join-Path $evidence 'summary.json')
}
