#Requires -Version 7.2
param(
    [Parameter(Mandatory)][string]$Archive,
    [string]$ReportPath = '',
    [ValidateRange(5,3600)][int]$TimeoutSeconds = 180,
    [ValidateRange(128,8192)][int]$DataMB = 256,
    [ValidateRange(384,16384)][int]$MemoryMB = 768
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'native-command.ps1')
if ($MemoryMB -lt $DataMB + 256) { throw 'MemoryMB must be at least DataMB + 256.' }
$image = 'postgres:17-bookworm'
$owner = [guid]::NewGuid().ToString('N')
$name = 'restivra-metadata-check-' + $owner
$started = [datetime]::UtcNow
$watch = [Diagnostics.Stopwatch]::StartNew()
$report = [ordered]@{schema_version=1;status='failed';started_at=$started.ToString('o');finished_at=$null;archive_sha256='';archive_bytes=0;postgres_image=$image;image_id='';restore='not_run';schema='not_run';cleanup='not_needed';counts=$null;error_code='';temporary_container=$name;worker_started=$false;scope='Isolated metadata restore and schema/count checks only; credentials, source archives and application recovery are separate.'}
$source = $null
$reserved = $null
$attempted = $false
$exitCode = 1
function Docker([string[]]$Arguments, [IO.Stream]$InputStream=$null) {
    $remaining = [Math]::Floor($TimeoutSeconds-$watch.Elapsed.TotalSeconds)
    if ($remaining -lt 1) { throw 'Metadata check exceeded its time limit.' }
    $commandBudget = if ($InputStream) { $remaining } else { [Math]::Min(30,$remaining) }
    $result = Invoke-RestivraNative -FilePath docker -Arguments $Arguments -TimeoutSeconds $commandBudget -InputStream $InputStream
    if ($result.ExitCode -ne 0 -or $result.Truncated) { throw 'Metadata Docker operation failed.' }
    return $result.Output.Trim()
}
try {
    if ($ReportPath) {
        $report.error_code='report_path_unavailable'
        $target = [IO.Path]::GetFullPath($ReportPath)
        # Reserve before restoring; never overwrite an existing report.
        $reserved = [IO.File]::Open($target,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
    }
    $report.error_code='archive_unavailable'
    $item = Get-Item -LiteralPath $Archive
    if ($item.PSIsContainer -or $item.Length -lt 5 -or $item.Length -gt 10GB) { $report.error_code='archive_invalid';throw 'Invalid metadata archive.' }
    $source = [IO.File]::Open($item.FullName,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::Read)
    $header = [byte[]]::new(5)
    if ($source.Read($header,0,5) -ne 5 -or [Text.Encoding]::ASCII.GetString($header) -ne 'PGDMP') { $report.error_code='archive_invalid';throw 'Expected a PostgreSQL custom archive.' }
    $source.Position=0
    $report.archive_sha256=[Convert]::ToHexString([Security.Cryptography.SHA256]::HashData($source)).ToLowerInvariant()
    $report.archive_bytes=$source.Length
    $source.Position=0
    $report.error_code='engine_not_local'
    $contextName=if($env:DOCKER_CONTEXT){$env:DOCKER_CONTEXT}else{Docker @('context','show')}
    if(-not $contextName){throw 'Docker context could not be resolved.'}
    $contextArgs=@('context','inspect',$contextName,'--format','{{.Endpoints.docker.Host}}')
    $endpoints=@(Docker $contextArgs)
    if($env:DOCKER_HOST){$endpoints+=$env:DOCKER_HOST}
    foreach($endpoint in $endpoints){if($endpoint -notmatch '^unix:///' -and $endpoint -notmatch '^npipe:////\./pipe/'){throw 'Select a local Docker Unix socket or Windows named pipe.'}}
    $report.error_code='engine_unavailable'
    if ((Docker @('info','--format','{{.OSType}}')) -ne 'linux') { throw 'Docker Linux engine required.' }
    $report.error_code='restore_image_missing'
    $report.image_id=Docker @('image','inspect',$image,'--format','{{.Id}}')
    $report.error_code='database_start_failed'
    $attempted=$true
    Docker @('run','--pull','never','--detach','--name',$name,'--network','none','--user','postgres','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges:true','--memory',"${MemoryMB}m",'--cpus','1','--pids-limit','128','--tmpfs',"/var/lib/postgresql/data:rw,size=${DataMB}m,uid=999,gid=999,mode=0700",'--tmpfs','/var/run/postgresql:rw,size=16m,uid=999,gid=999,mode=0755','--tmpfs','/tmp:rw,size=16m,mode=1777','--label','restivra.metadata-check=true','--label',"restivra.metadata-owner=$owner",'--env','POSTGRES_HOST_AUTH_METHOD=trust',$report.image_id)|Out-Null
    $ready=$false
    while ($watch.Elapsed.TotalSeconds -lt $TimeoutSeconds) {
        $remaining=[Math]::Floor($TimeoutSeconds-$watch.Elapsed.TotalSeconds)
        if($remaining -lt 1){break}
        $result=Invoke-RestivraNative -FilePath docker -Arguments @('exec',$name,'sh','-c','test "$(cat /proc/1/comm)" = postgres && pg_isready -q -U postgres') -TimeoutSeconds ([Math]::Min(5,$remaining))
        if ($result.ExitCode -eq 0) { $ready=$true;break }
        Start-Sleep -Milliseconds 250
    }
    if (-not $ready) { $report.error_code='database_start_failed';throw 'Temporary metadata database did not become ready.' }
    $report.restore='failed'
    $report.error_code='metadata_restore_failed'
    Docker @('exec','-i',$name,'pg_restore','-U','postgres','--no-owner','--no-acl','--exit-on-error','-d','postgres') $source|Out-Null
    $report.restore='passed'
    $report.schema='failed'
    $report.error_code='metadata_schema_invalid'
    $schema=Docker @('exec',$name,'psql','-X','-At','-U','postgres','-v','ON_ERROR_STOP=1','-c',"SELECT to_regclass('public.projects') IS NOT NULL AND to_regclass('public.jobs') IS NOT NULL AND to_regclass('public.workers') IS NOT NULL AND to_regclass('public.installation') IS NOT NULL AND to_regclass('public.job_attempts') IS NOT NULL")
    if ($schema -ne 't') { throw 'Archive does not contain the supported Restivra metadata schema.' }
    $countsSQL="SELECT json_build_object('projects',(SELECT count(*) FROM projects),'jobs',(SELECT count(*) FROM jobs),'reports',(SELECT count(*) FROM jobs WHERE report IS NOT NULL),'attempts',(SELECT count(*) FROM job_attempts),'active_jobs',(SELECT count(*) FROM jobs WHERE status IN ('running','queued')),'orphan_jobs',(SELECT count(*) FROM jobs j LEFT JOIN projects p ON p.id=j.project_id WHERE p.id IS NULL),'orphan_attempts',(SELECT count(*) FROM job_attempts a LEFT JOIN jobs j ON j.id=a.job_id WHERE j.id IS NULL),'valid_installation',(SELECT count(*)=1 AND bool_and(id ~ '^[a-f0-9]{32}$') FROM installation))"
    $counts=(Docker @('exec',$name,'psql','-X','-At','-U','postgres','-v','ON_ERROR_STOP=1','-c',$countsSQL))|ConvertFrom-Json
    if (-not $counts.valid_installation -or $counts.orphan_jobs -ne 0 -or $counts.orphan_attempts -ne 0) { throw 'Archive identity or metadata relationships are invalid.' }
    $report.counts=$counts
    $report.schema='passed'
    $source.Position=0
    if ([Convert]::ToHexString([Security.Cryptography.SHA256]::HashData($source)).ToLowerInvariant() -ne $report.archive_sha256) { $report.error_code='archive_changed';throw 'Archive changed during verification.' }
    $report.status='passed'
    $report.error_code=''
    $exitCode=0
} catch {
    if($_.Exception.Message -in @('Native command exceeded its time limit.','Metadata check exceeded its time limit.')){$report.error_code='check_timeout'}
    if (-not $report.error_code) { $report.error_code='metadata_check_failed' }
} finally {
    if ($source) { $source.Dispose() }
    if ($attempted) {
        $report.cleanup='failed'
        try {
            $ownership=Invoke-RestivraNative -FilePath docker -Arguments @('inspect','--format','{{index .Config.Labels "restivra.metadata-owner"}}',$name) -TimeoutSeconds 10
            if($ownership.ExitCode -ne 0 -or $ownership.Output.Trim() -ne $owner){throw 'Temporary container ownership could not be verified.'}
            $remove=Invoke-RestivraNative -FilePath docker -Arguments @('rm','--force','--volumes',$name) -TimeoutSeconds 20
            if ($remove.ExitCode -eq 0) { $report.cleanup='passed' }
        } catch {}
        if ($report.cleanup -ne 'passed') { $report.status='failed';$report.error_code='cleanup_failed';$exitCode=1 }
    }
    $report.finished_at=[datetime]::UtcNow.ToString('o')
    $text=($report|ConvertTo-Json -Depth 7)+"`n"
    if ($reserved) {
        try { $bytes=[Text.UTF8Encoding]::new($false).GetBytes($text);$reserved.Write($bytes,0,$bytes.Length);$reserved.Flush($true) }
        catch {$report.status='failed';$report.error_code='report_write_failed';$exitCode=1;$text=($report|ConvertTo-Json -Depth 7)+"`n"}
        finally { $reserved.Dispose() }
    }
}
Write-Output $text
exit $exitCode
