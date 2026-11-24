#Requires -Version 7.2
# Real PostgreSQL export failures in a disposable, isolated Compose project.
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'native-command.ps1')
$root=Split-Path -Parent $PSScriptRoot
$suffix=[guid]::NewGuid().ToString('N')
$project="restivra-export-test-$suffix"
$directory=Join-Path $root "work/metadata-export-$suffix"
$fixtureScripts=Join-Path $directory 'scripts'
New-Item -ItemType Directory -Path $fixtureScripts|Out-Null
Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'metadata-backup.ps1'),(Join-Path $PSScriptRoot 'native-command.ps1'),(Join-Path $PSScriptRoot 'private-io.ps1') -Destination $fixtureScripts
$compose=@'
services:
  db:
    image: postgres:17-bookworm
    pull_policy: never
    user: postgres
    network_mode: none
    read_only: true
    cap_drop: [ALL]
    security_opt: [no-new-privileges:true]
    mem_limit: 512m
    pids_limit: 128
    environment:
      POSTGRES_USER: restivra
      POSTGRES_DB: restivra
      POSTGRES_HOST_AUTH_METHOD: trust
    tmpfs:
      - /var/lib/postgresql/data:rw,size=128m,uid=999,gid=999,mode=0700
      - /var/run/postgresql:rw,size=16m,uid=999,gid=999,mode=0755
      - /tmp:rw,size=16m,mode=1777
    healthcheck:
      test: [CMD-SHELL, 'test "$$(cat /proc/1/comm)" = postgres && pg_isready -q -U restivra']
      interval: 1s
      timeout: 2s
      retries: 45
'@
[IO.File]::WriteAllText((Join-Path $directory 'compose.yaml'),$compose)
$executable=(Get-Process -Id $PID).Path
$command=Join-Path $fixtureScripts 'metadata-backup.ps1'
$previousProject=$env:COMPOSE_PROJECT_NAME
$records=[Collections.Generic.List[object]]::new()
function Docker([string[]]$Arguments) {
 $result=Invoke-RestivraNative -FilePath docker -WorkingDirectory $directory -Arguments $Arguments -TimeoutSeconds 60
 if($result.ExitCode -ne 0 -or $result.Truncated){throw 'Export fixture Docker operation failed'}
 return $result.Output.Trim()
}
function Export([string]$Path,[int]$Timeout=30,[int]$MaxMB=16) {
 $watch=[Diagnostics.Stopwatch]::StartNew()
 $result=Invoke-RestivraNative -FilePath $executable -Arguments @('-NoProfile','-NonInteractive','-File',$command,'-OutputPath',$Path,'-TimeoutSeconds',"$Timeout",'-MaxArchiveMB',"$MaxMB") -TimeoutSeconds 45
 return @{exit=$result.ExitCode;elapsed_seconds=$watch.Elapsed.TotalSeconds}
}
try {
 $env:COMPOSE_PROJECT_NAME=$project
 Docker @('compose','-p',$project,'up','-d','--wait','--wait-timeout','60')|Out-Null
 $container=Docker @('compose','-p',$project,'ps','--quiet','db')
 Docker @('exec',$container,'psql','-X','-U','restivra','-d','restivra','-v','ON_ERROR_STOP=1','-c','CREATE TABLE backup_fixture AS SELECT i, md5(random()::text) || md5(random()::text) AS payload FROM generate_series(1,100000) AS i')|Out-Null
 $complete=Join-Path $directory 'completed metadata.dump'
 $valid=Export $complete
 if($valid.exit -ne 0 -or -not (Test-Path -LiteralPath $complete) -or (Test-Path -LiteralPath "$complete.partial")){throw 'Completed export was not published exclusively'}
 if((Get-Item -LiteralPath $complete).Length -le 1MB){throw 'Fixture must exceed the tested size limit'}
 $hash=(Get-FileHash -LiteralPath $complete).Hash
 Docker @('exec',$container,'createdb','-U','restivra','restored')|Out-Null
 $stream=[IO.File]::OpenRead($complete)
 try {
  $restored=Invoke-RestivraNative -FilePath docker -Arguments @('exec','-i',$container,'pg_restore','-U','restivra','--no-owner','--no-acl','--exit-on-error','-d','restored') -InputStream $stream -TimeoutSeconds 45
  if($restored.ExitCode -ne 0){throw 'Streamed export did not restore'}
 } finally {$stream.Dispose()}
 if((Docker @('exec',$container,'psql','-X','-At','-U','restivra','-d','restored','-c','SELECT count(*) FROM backup_fixture')) -ne '100000'){throw 'Export/restore changed the fixture'}
 $records.Add(@{case='binary_export_and_restore';passed=$true;rows=100000;archive_bytes=(Get-Item -LiteralPath $complete).Length})
 if((Export $complete).exit -eq 0 -or (Get-FileHash -LiteralPath $complete).Hash -ne $hash){throw 'Existing completed export was replaced'}
 $records.Add(@{case='completed_non_overwrite';passed=$true})
 $reserved=Join-Path $directory 'reserved.dump'
 [IO.File]::WriteAllText("$reserved.partial",'preserve-existing-partial')
 $partialHash=(Get-FileHash -LiteralPath "$reserved.partial").Hash
 if((Export $reserved).exit -eq 0 -or (Get-FileHash -LiteralPath "$reserved.partial").Hash -ne $partialHash -or (Test-Path -LiteralPath $reserved)){throw 'Existing partial export was replaced'}
 $records.Add(@{case='partial_non_overwrite';passed=$true})
 $limited=Join-Path $directory 'size limited.dump'
 $size=Export $limited 30 1
 if($size.exit -eq 0 -or (Test-Path -LiteralPath $limited) -or -not (Test-Path -LiteralPath "$limited.partial") -or (Get-Item -LiteralPath "$limited.partial").Length -gt 1MB){throw 'Oversized export was published or exceeded its disk bound'}
 $records.Add(@{case='size_limit_keeps_partial';passed=$true;elapsed_seconds=$size.elapsed_seconds;partial_bytes=(Get-Item -LiteralPath "$limited.partial").Length})
 Docker @('exec','--detach','--env','PGAPPNAME=restivra-export-lock',$container,'psql','-X','-U','restivra','-d','restivra','-v','ON_ERROR_STOP=1','-c',"BEGIN; LOCK TABLE backup_fixture IN ACCESS EXCLUSIVE MODE; SELECT pg_sleep(12); COMMIT")|Out-Null
 $locked=$false
 for($i=0;$i -lt 30;$i++){
  $locked=(Docker @('exec',$container,'psql','-X','-At','-U','restivra','-d','restivra','-c',"SELECT EXISTS (SELECT 1 FROM pg_locks WHERE relation='backup_fixture'::regclass AND mode='AccessExclusiveLock' AND granted)")) -eq 't'
  if($locked){break}
  Start-Sleep -Milliseconds 100
 }
 if(-not $locked){throw 'Timeout fixture did not acquire its table lock'}
 $blocked=Join-Path $directory 'blocked.dump'
 $timeout=Export $blocked 2
 if($timeout.exit -eq 0 -or $timeout.elapsed_seconds -gt 10 -or (Test-Path -LiteralPath $blocked) -or -not (Test-Path -LiteralPath "$blocked.partial")){throw 'Locked export did not fail within the bounded timeout'}
 $records.Add(@{case='table_lock_timeout_keeps_partial';passed=$true;elapsed_seconds=$timeout.elapsed_seconds})
 $noDump=$false
 for($i=0;$i -lt 30;$i++){
  $noDump=(Docker @('exec',$container,'psql','-X','-At','-U','restivra','-d','restivra','-c',"SELECT NOT EXISTS (SELECT 1 FROM pg_stat_activity WHERE application_name='pg_dump')")) -eq 't'
  if($noDump){break}
  Start-Sleep -Milliseconds 500
 }
 if(-not $noDump){throw 'Failed export left a pg_dump connection'}
 @{passed=$true;verified_at=[datetime]::UtcNow.ToString('o');cases=@($records);failed_export_connections=0;cleanup='pending'}|ConvertTo-Json -Depth 5|Set-Content (Join-Path $directory 'summary.json')
 Write-Host 'Metadata export checks passed: actual restore, exclusive files, byte limit and locked-table timeout.'
} finally {
 $cleanup=Invoke-RestivraNative -FilePath docker -WorkingDirectory $directory -Arguments @('compose','-p',$project,'down') -TimeoutSeconds 45
 $env:COMPOSE_PROJECT_NAME=$previousProject
 if($cleanup.ExitCode -ne 0){throw "Export fixture cleanup failed: $project"}
 $summaryPath=Join-Path $directory 'summary.json'
 if(Test-Path -LiteralPath $summaryPath){$summary=Get-Content -LiteralPath $summaryPath -Raw|ConvertFrom-Json;$summary.cleanup='passed';$summary|ConvertTo-Json -Depth 5|Set-Content -LiteralPath $summaryPath}
}
