#Requires -Version 7.2
# Docker-only real PostgreSQL smoke test: unique synthetic resources, no Go/Node.
param(
    [string]$Base = 'http://127.0.0.1:7474',
    [string]$EnvFile = (Join-Path (Split-Path -Parent $PSScriptRoot) '.env')
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'api-session.ps1')
$root = Split-Path -Parent $PSScriptRoot
$suffix = [guid]::NewGuid().ToString('N')
$source = "restivra-pilot-source-$suffix"
$backupDirectory = Join-Path $root "backups/pilot-$suffix"
$evidenceDirectory = Join-Path $root "work/pilot-$suffix"
$sourceCreated = $false
$jobs = [Collections.Generic.List[object]]::new()

function Invoke-PilotDocker([string[]]$Arguments) {
    $result = & docker @Arguments
    if ($LASTEXITCODE -ne 0) { throw "Synthetic fixture Docker operation failed: $($Arguments[0])" }
    return $result
}
function API([string]$Path, [string]$Method = 'Get', $Body = $null) {
    $options = @{Uri="$Base/api$Path"; Method=$Method; WebSession=$connection.WebSession; Headers=$connection.Headers; TimeoutSec=20; MaximumRedirection=0}
    if ($Method -ne 'Get') { $options.ContentType='application/json'; $options.Body=$Body|ConvertTo-Json -Depth 12 -Compress }
    Invoke-RestMethod @options
}

Push-Location $root
try {
    & (Join-Path $PSScriptRoot 'pilot-check.ps1') -Base $Base -EnvFile $EnvFile
    $connection = New-RestivraSession -Base $Base -EnvFile $EnvFile
    New-Item -ItemType Directory -Path $backupDirectory,$evidenceDirectory | Out-Null
    Invoke-PilotDocker @('run','--detach','--name',$source,'--network','none','--memory','512m','--cpus','1','--pids-limit','128','--label','restivra.pilot-fixture=true','--env','POSTGRES_HOST_AUTH_METHOD=trust','postgres:17-bookworm') | Out-Null
    $sourceCreated = $true
    $ready = $false
    for ($attempt=0; $attempt -lt 90; $attempt++) {
        & docker exec $source sh -c 'test "$(cat /proc/1/comm)" = postgres && pg_isready -q -U postgres' 2>$null
        if ($LASTEXITCODE -eq 0) { $ready=$true; break }
        Start-Sleep -Milliseconds 500
    }
    if (-not $ready) { throw 'Synthetic PostgreSQL readiness timed out.' }
    Invoke-PilotDocker @('exec',$source,'psql','-X','-U','postgres','-v','ON_ERROR_STOP=1','-c',"CREATE TABLE customers(id integer PRIMARY KEY, name text NOT NULL); INSERT INTO customers VALUES (1, 'Ada'), (2, 'Deniz');") | Out-Null
    Invoke-PilotDocker @('exec',$source,'pg_dump','-U','postgres','-Fc','-f','/tmp/sample.dump','postgres') | Out-Null
    Invoke-PilotDocker @('cp',"${source}:/tmp/sample.dump",(Join-Path $backupDirectory 'sample.dump.partial')) | Out-Null
    [IO.File]::Move((Join-Path $backupDirectory 'sample.dump.partial'),(Join-Path $backupDirectory 'sample.dump'))
    foreach ($sample in @(
        @{name='Passing sample';sql='SELECT count(*) = 2 FROM public.customers';expected='passed'},
        @{name='Intentional failure';sql='SELECT false';expected='failed'}
    )) {
        $config = $connection.Session.default_config | ConvertTo-Json -Depth 10 | ConvertFrom-Json
        $config.checks = @(@{name=$sample.name;sql=$sample.sql})
        $project = API '/projects' 'Post' @{name="Pilot $($suffix.Substring(0,8)) / $($sample.name)";backup_pattern="pilot-$suffix/sample.dump";schedule_time='';timezone='UTC';config=$config}
        $job = API '/jobs' 'Post' @{project_id=$project.id}
        $jobs.Add(@{project_id=$project.id;job_id=$job.id})
        $deadline = [datetime]::UtcNow.AddMinutes(8)
        do {
            $job = API "/jobs/$($job.id)"
            if ($job.status -notin @('queued','running')) { break }
            Start-Sleep -Milliseconds 500
        } while ([datetime]::UtcNow -lt $deadline)
        if ($job.status -ne $sample.expected -or $job.report.restore_status -ne 'passed' -or $job.report.cleanup_status -ne 'passed') { throw "Unexpected synthetic outcome for $($sample.name). Inspect its run in the panel." }
        $checkStatus = if($sample.expected -eq 'passed'){'passed'}else{'failed'}
        if ($job.report.checks[0].status -ne $checkStatus) { throw 'The synthetic SQL check did not produce the expected result.' }
        $left = @(Invoke-PilotDocker @('ps','-aq','--filter',"label=restivra.run=$($job.id)"))
        if ($left.Count -ne 0) { throw 'A synthetic restore container remains. Inspect worker health before continuing.' }
        $job | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $evidenceDirectory "$($sample.expected).json")
        Write-Host "$($sample.name): $($job.status); restore and cleanup passed."
    }
    [ordered]@{passed=$true;synthetic_data=$true;api_version=$connection.Session.version;projects=$jobs;backup_pattern="pilot-$suffix/sample.dump";verified_at=[datetime]::UtcNow.ToString('o')} | ConvertTo-Json -Depth 6 | Set-Content (Join-Path $evidenceDirectory 'summary.json')
    Write-Host "Pilot smoke passed. Inspect both Pilot projects in $Base. Schedules are disabled."
    Write-Host "Local evidence: $evidenceDirectory"
    Write-Host 'Archive the two sample projects in the panel after inspection. Keep their sample dump until then.'
} finally {
    # Never cancel or remove another installation's work. On failure preserve
    # created projects/archive for inspection, even if a job is still active.
    if ($sourceCreated) {
        & docker rm -f -v $source | Out-Null
        if ($LASTEXITCODE -ne 0) { Write-Warning "Synthetic fixture cleanup failed: $source" }
    }
    Pop-Location
}
