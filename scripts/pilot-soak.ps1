#Requires -Version 7.2
param(
    [Parameter(Mandatory)][string]$ProjectDirectory,
    [string]$ComposeProject = 'restivra-pilot-20261117',
    [string]$Base = 'http://127.0.0.1:17474',
    [int]$Minutes = 120,
    [string]$EvidenceDirectory = 'work/pilot-soak'
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'api-session.ps1')
if ($ComposeProject -notmatch '^restivra-pilot-[a-z0-9-]+$') { throw 'Use an isolated restivra-pilot-* Compose project.' }
if ($Minutes -lt 1 -or $Minutes -gt 180) { throw 'Minutes must be 1–180.' }
New-Item -ItemType Directory -Path $EvidenceDirectory -Force | Out-Null
$evidence = (Resolve-Path -LiteralPath $EvidenceDirectory).Path
$connection = New-RestivraSession -Base $Base -EnvFile (Join-Path $ProjectDirectory '.env')
$headers = $connection.Headers
$web = $connection.WebSession
$started = [DateTime]::UtcNow
$deadline = $started.AddMinutes($Minutes)
$records = [Collections.Generic.List[object]]::new()
function API([string]$Path,[string]$Method='Get',$Body=$null) {
    $options=@{Uri="$Base/api$Path";Method=$Method;WebSession=$web;Headers=$headers;TimeoutSec=15;MaximumRedirection=0}
    if ($Method -ne 'Get') { $options.ContentType='application/json';$options.Body=if($null -eq $Body){'{}'}else{$Body|ConvertTo-Json -Depth 12 -Compress} }
    Invoke-RestMethod @options
}
function DC([string[]]$Arguments) {
    $output=& docker compose --project-directory $ProjectDirectory -p $ComposeProject @Arguments
    if($LASTEXITCODE -ne 0){throw "Isolated Compose command failed: $($Arguments[0])"}
    return $output
}
function Project([string]$Name,[string]$Pattern,[string]$SQL) {
    $config=$connection.Session.default_config|ConvertTo-Json -Depth 8|ConvertFrom-Json
    $config.checks=@(@{name=$Name;sql=$SQL})
    return API '/projects' 'Post' @{name=$Name;backup_pattern=$Pattern;config=$config;schedule_time='';timezone='UTC'}
}
function Wait-Job([string]$ID) {
    $limit=[DateTime]::UtcNow.AddMinutes(4)
    do { $job=API "/jobs/$ID";if($job.status -notin @('queued','running')){return $job};Start-Sleep -Milliseconds 500 } while([DateTime]::UtcNow -lt $limit)
    throw 'A soak job exceeded the test deadline.'
}
function Assert-Job($Job,[string]$Expected,[string]$Scenario) {
    if($Job.status -ne $Expected){throw "$Scenario expected $Expected; got $($Job.status)"}
    if($Job.report -and $Job.report.cleanup_status -notin @('passed','not_needed')){throw "$Scenario cleanup failed"}
    $containers=& docker ps -aq --filter "label=restivra.run=$($Job.id)"
    if($containers){throw "$Scenario leaked a restore container"}
    $record=[pscustomobject][ordered]@{scenario=$Scenario;id=$Job.id;status=$Job.status;cleanup=$Job.report.cleanup_status;duration_ms=$Job.report.duration_ms;error_code=$Job.report.error_code;finished_at=$Job.finished_at}
    $records.Add($record)
    $record|ConvertTo-Json -Compress|Add-Content -LiteralPath (Join-Path $evidence 'runs.ndjson')
}
function Run-Project($Project,[string]$Expected,[string]$Scenario) {
    $job=API '/jobs' 'Post' @{project_id=$Project.id}
    Assert-Job (Wait-Job $job.id) $Expected $Scenario
}
$good=Project 'Pilot commerce' 'commerce.dump' 'SELECT count(*) = 100000 FROM public.order_items'
$falseProject=Project 'Pilot false check' 'commerce.dump' 'SELECT false'
$missing=Project 'Pilot missing archive' 'missing.dump' 'SELECT true'
$bad=Project 'Pilot corrupt archive' 'bad.dump' 'SELECT true'
$slow=Project 'Pilot interruption' 'commerce.dump' 'SELECT true FROM (SELECT pg_sleep(8)) AS waited'
$iteration=0
$failure=$null
try {
    # This schedule is intentionally near-future and uses real wall-clock dispatch.
    $schedule=Project 'Pilot scheduled' 'commerce.dump' 'SELECT count(*) = 25000 FROM public.orders'
    $schedule.schedule_time=[DateTime]::UtcNow.AddMinutes(2).ToString('HH:mm')
    $schedule=API "/projects/$($schedule.id)" 'Put' $schedule
    do {
        $iteration++
        Run-Project $good 'passed' 'repeat_restore'
        if($iteration % 10 -eq 1){ Run-Project $falseProject 'failed' 'false_check';Run-Project $missing 'failed' 'missing_archive';Run-Project $bad 'failed' 'corrupt_archive' }
        if($iteration -eq 3 -or $iteration -eq 30) {
            $job=API '/jobs' 'Post' @{project_id=$slow.id}
            $limit=[DateTime]::UtcNow.AddSeconds(30)
            do {$current=API "/jobs/$($job.id)";if($current.status -eq 'running'){break};Start-Sleep -Milliseconds 200}while([DateTime]::UtcNow -lt $limit)
            if($current.status -ne 'running'){throw 'Could not interrupt a running job'}
            DC @('kill','--signal','SIGKILL','worker')|Out-Null
            # Restart the same container/image: 'up' can silently recreate it
            # after an unrelated local presentation build moves an image tag.
            DC @('start','worker')|Out-Null
            $recovered=Wait-Job $job.id
            Assert-Job $recovered 'passed' 'worker_sigkill_recovered'
            if($recovered.attempt_count -lt 2 -or $recovered.attempts[0].status -ne 'interrupted'){throw 'Worker recovery lost prior attempt evidence'}
            Run-Project $good 'passed' 'restore_after_worker_recovery'
        }
        if($iteration -eq 5 -or $iteration -eq 45) {
            DC @('restart','db')|Out-Null
            $ready=$false
            for($attempt=0;$attempt -lt 60;$attempt++) {
                try { $h=API '/health';if($h.worker_online){$ready=$true;break} }catch{}
                Start-Sleep -Seconds 1
            }
            if(-not $ready){throw 'Services did not recover after database restart'}
            Run-Project $good 'passed' 'restore_after_database_restart'
        }
        if($iteration -eq 7) {
            $job=API '/jobs' 'Post' @{project_id=$good.id}
            $limit=[DateTime]::UtcNow.AddSeconds(30)
            do {$current=API "/jobs/$($job.id)";if($current.status -eq 'running'){break};Start-Sleep -Milliseconds 200}while([DateTime]::UtcNow -lt $limit)
            if($current.status -ne 'running'){throw 'Could not exercise a running job during database stall'}
            DC @('pause','db')|Out-Null
            try { Start-Sleep -Seconds 8 } finally { DC @('unpause','db')|Out-Null }
            Assert-Job (Wait-Job $job.id) 'passed' 'database_eight_second_stall'
        }
        if($iteration % 10 -eq 0) {
            $job=API '/jobs' 'Post' @{project_id=$slow.id}
            API "/jobs/$($job.id)/cancel" 'Post'|Out-Null
            Assert-Job (Wait-Job $job.id) 'cancelled' 'cancel'
        }
        $records.Count|Out-Host
        $remaining=($deadline-[DateTime]::UtcNow).TotalSeconds
        if($remaining -gt 0){Start-Sleep -Seconds ([Math]::Min(45,[Math]::Ceiling($remaining)))}
    }while([DateTime]::UtcNow -lt $deadline)
    # Invoke-RestMethod can emit a JSON array as one pipeline object. Assign
    # first so Where-Object examines jobs rather than the array wrapper.
    $projectJobs=API "/jobs?project_id=$($schedule.id)"
    $scheduled=@($projectJobs|Where-Object trigger -eq 'schedule')
    if($Minutes -ge 3 -and $scheduled.Count -ne 1){throw 'Expected exactly one real scheduled job'}
    foreach($job in $scheduled){Assert-Job (Wait-Job $job.id) 'passed' 'wall_clock_schedule'}
}catch{$failure=$_.Exception.Message;throw}
finally {
    # Disable the synthetic future schedule even when a test fails.
    if($schedule){try{$schedule.schedule_time='';API "/projects/$($schedule.id)" 'Put' $schedule|Out-Null}catch{}}
    $finished=[DateTime]::UtcNow
    [ordered]@{started_at=$started.ToString('o');finished_at=$finished.ToString('o');elapsed_minutes=[Math]::Round(($finished-$started).TotalMinutes,2);iterations=$iteration;runs=$records.Count;failure=$failure;outcomes=@($records|Group-Object status|ForEach-Object{@{status=$_.Name;count=$_.Count}})}|ConvertTo-Json -Depth 10|Set-Content -LiteralPath (Join-Path $evidence 'summary.json')
}
