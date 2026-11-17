# Real PostgreSQL / pg_dump / live API acceptance run. Synthetic data only.
# Requires the Compose app to be running. Source and restore containers have no ports.
$ErrorActionPreference = 'Stop'
$projectDir = Split-Path -Parent $PSScriptRoot
$runSuffix = [guid]::NewGuid().ToString('N').Substring(0,8)
$source = "restivra-commerce-source-$runSuffix"
$base = 'http://127.0.0.1:7474'
$evidenceDir = Join-Path $projectDir "work/acceptance-$runSuffix"
$backupDir = Join-Path $projectDir "backups/acceptance-$runSuffix"
New-Item -ItemType Directory -Path $evidenceDir,$backupDir -Force | Out-Null
$results = [Collections.Generic.List[object]]::new()
$createdProjects = [Collections.Generic.List[object]]::new()
$sourceCreated = $false

function Invoke-DockerChecked {
    param([string[]]$DockerArgs)
    $output = & docker @DockerArgs
    if ($LASTEXITCODE -ne 0) { throw "Docker command failed: $($DockerArgs[0])" }
    return $output
}
function Invoke-SourceSQL {
    param([string]$SQL)
    return (Invoke-DockerChecked @('exec',$source,'psql','-X','-qAt','-U','postgres','-d','commerce','-v','ON_ERROR_STOP=1','-c',$SQL)) -join "`n"
}
function New-VerificationProject {
    param([string]$Name,[string]$Pattern,[object[]]$Checks)
    $config = $session.default_config | ConvertTo-Json -Depth 10 | ConvertFrom-Json
    $config.timeout_seconds = 180
    $config.checks = @($Checks)
    $body = @{name="Lab $runSuffix / $Name";backup_pattern=$Pattern;config=$config;schedule_time='';timezone='UTC'} | ConvertTo-Json -Depth 10
    $project = Invoke-RestMethod "$base/api/projects" -Method Post -Headers $headers -ContentType 'application/json' -Body $body
    $createdProjects.Add($project)
    return $project
}
function Wait-Verification {
    param([string]$JobID)
    $deadline = [DateTime]::UtcNow.AddMinutes(4)
    do {
        $job = Invoke-RestMethod "$base/api/jobs/$JobID"
        if ($job.status -notin @('queued','running')) { return $job }
        Start-Sleep -Milliseconds 400
    } while ([DateTime]::UtcNow -lt $deadline)
    throw "Job $JobID did not finish before the acceptance deadline"
}
function Start-Verification {
    param([object]$Project)
    $body = @{project_id=$Project.id} | ConvertTo-Json
    return Invoke-RestMethod "$base/api/jobs" -Method Post -Headers $headers -ContentType 'application/json' -Body $body
}
function Assert-Verification {
    param([string]$Scenario,[object]$Job,[string]$Status,[string]$CheckStatus='')
    if ($Job.status -ne $Status) { throw "$Scenario expected $Status, got $($Job.status): $($Job.error)" }
    if ($CheckStatus -and $Job.report.checks[0].status -ne $CheckStatus) { throw "$Scenario returned an unexpected check result" }
    if ($null -eq $Job.report -or $Job.report.cleanup_status -ne 'passed') { throw "$Scenario did not report successful cleanup" }
    $remaining = Invoke-DockerChecked @('ps','-a','-q','--filter',"label=restivra.run=$($Job.id)")
    if ($remaining) { throw "$Scenario leaked a restore container" }
    $Job | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $evidenceDir "$Scenario.json") -Encoding utf8
    $results.Add([ordered]@{scenario=$Scenario;job_id=$Job.id;status=$Job.status;restore=$Job.report.restore_status;checks=@($Job.report.checks);cleanup=$Job.report.cleanup_status;duration_ms=$Job.report.duration_ms;sha256=$Job.report.backup.sha256})
    Write-Host "$Scenario : $($Job.status), restore=$($Job.report.restore_status), cleanup=$($Job.report.cleanup_status)"
}

Push-Location $projectDir
try {
    $session = Invoke-RestMethod "$base/api/session"
    $headers = @{'X-CSRF-Token'=$session.csrf_token}
    $health = Invoke-RestMethod "$base/api/health"
    if (-not $health.worker_online) { throw 'Start the Compose worker before acceptance testing.' }
    Invoke-DockerChecked @('run','-d','--name',$source,'--network','none','--memory','768m','--cpus','1','--pids-limit','128','--env','POSTGRES_HOST_AUTH_METHOD=trust','--env','POSTGRES_DB=commerce','postgres:17-bookworm') | Out-Null
    $sourceCreated = $true
    $ready = $false
    for ($attempt=0;$attempt -lt 120;$attempt++) {
        & docker exec $source sh -c 'test "$(cat /proc/1/comm)" = postgres && pg_isready -q -U postgres -d commerce' 2>$null
        if ($LASTEXITCODE -eq 0) { $ready=$true;break }
        Start-Sleep -Milliseconds 500
    }
    if (-not $ready) { throw 'Source PostgreSQL did not become ready.' }
    Invoke-DockerChecked @('cp',(Join-Path $projectDir 'testdata/commerce.sql'),"${source}:/tmp/commerce.sql") | Out-Null
    Invoke-DockerChecked @('exec',$source,'psql','-X','-U','postgres','-d','commerce','-v','ON_ERROR_STOP=1','-f','/tmp/commerce.sql') | Out-Null
    $metricsSQL = "SELECT json_build_object('customers',(SELECT count(*) FROM customers),'products',(SELECT count(*) FROM products),'orders',(SELECT count(*) FROM orders),'order_items',(SELECT count(*) FROM order_items),'revenue',(SELECT sum(total) FROM order_totals),'content_hash',(SELECT md5(string_agg(order_id::text||':'||line_no::text||':'||quantity::text||':'||unit_price::text,',' ORDER BY order_id,line_no)) FROM order_items),'version',version())"
    $metrics = Invoke-SourceSQL $metricsSQL | ConvertFrom-Json
    if ($metrics.orders -ne 25000 -or $metrics.order_items -ne 100000) { throw 'Source dataset is incomplete.' }
    $metrics | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath (Join-Path $evidenceDir 'source-metrics.json') -Encoding utf8
    Invoke-DockerChecked @('exec',$source,'pg_dump','-U','postgres','-d','commerce','-Fc','-f','/tmp/commerce.dump') | Out-Null
    $partial = Join-Path $backupDir 'commerce.partial'
    $archive = Join-Path $backupDir 'commerce.dump'
    Invoke-DockerChecked @('cp',"${source}:/tmp/commerce.dump",$partial) | Out-Null
    # Only this newly-created, exact path is renamed; there is no recursive move.
    Move-Item -LiteralPath $partial -Destination $archive
    $pattern = "acceptance-$runSuffix/commerce.dump"
    $checks = @(
        @{name='10,000 customers';sql='SELECT count(*) = 10000 FROM public.customers'},
        @{name='500 products';sql='SELECT count(*) = 500 FROM public.products'},
        @{name='25,000 orders';sql='SELECT count(*) = 25000 FROM public.orders'},
        @{name='100,000 order items';sql='SELECT count(*) = 100000 FROM public.order_items'},
        @{name='Foreign keys restored';sql="SELECT count(*) = 3 FROM pg_constraint WHERE contype = 'f' AND connamespace = 'public'::regnamespace"},
        @{name='Revenue view matches source';sql="SELECT sum(total) = $($metrics.revenue) FROM public.order_totals"},
        @{name='Function restored';sql='SELECT public.order_total(1) > 0'},
        @{name='Identity sequence restored';sql='SELECT last_value = 25000 FROM public.orders_id_seq'},
        @{name='Indexes restored';sql="SELECT count(*) = 2 FROM pg_indexes WHERE schemaname='public' AND indexname IN ('orders_customer_id_idx','customers_profile_idx')"},
        @{name='JSONB data restored';sql="SELECT count(*) = 5000 FROM public.customers WHERE profile->>'region' = 'TR'"},
        @{name='Data fingerprint matches source';sql="SELECT md5(string_agg(order_id::text||':'||line_no::text||':'||quantity::text||':'||unit_price::text,',' ORDER BY order_id,line_no)) = '$($metrics.content_hash)' FROM public.order_items"}
    )
    $good = New-VerificationProject 'Commerce restore' $pattern $checks
    $job = Wait-Verification (Start-Verification $good).id
    Assert-Verification '01-relational-restore' $job 'passed'
    # Simulate data loss only in the dedicated synthetic source container.
    Invoke-SourceSQL 'TRUNCATE public.order_items, public.orders' | Out-Null
    $sourceOrders = Invoke-SourceSQL 'SELECT count(*) FROM public.orders'
    if ($sourceOrders -ne '0') { throw 'Synthetic source data-loss step failed.' }
    $recovered = Wait-Verification (Start-Verification $good).id
    Assert-Verification '02-source-empty-backup-restores' $recovered 'passed'
    if (-not ($recovered.report.warnings | Where-Object { $_ -like '*identical to the previous*' })) { throw 'Unchanged backup warning was not recorded.' }
    $wrong = New-VerificationProject 'Wrong business expectation' $pattern @(@{name='Wrong count';sql='SELECT count(*) = 1 FROM public.orders'})
    Assert-Verification '03-false-business-check' (Wait-Verification (Start-Verification $wrong).id) 'failed' 'failed'
    $missing = New-VerificationProject 'Missing table' $pattern @(@{name='Missing table';sql='SELECT count(*) > 0 FROM public.table_that_does_not_exist'})
    Assert-Verification '04-query-error' (Wait-Verification (Start-Verification $missing).id) 'failed' 'error'
    $write = New-VerificationProject 'Read-only verification' $pattern @(@{name='Write attempt';sql='SELECT lo_create(654321) > 0'})
    Assert-Verification '05-write-blocked' (Wait-Verification (Start-Verification $write).id) 'failed' 'error'
    $slow = New-VerificationProject 'Query timeout' $pattern @(@{name='Slow query';sql='SELECT pg_sleep(15) IS NULL'})
    Assert-Verification '06-query-timeout' (Wait-Verification (Start-Verification $slow).id) 'failed' 'error'
    $archiveBytes = [IO.File]::ReadAllBytes($archive)
    [IO.File]::WriteAllBytes((Join-Path $backupDir 'truncated.dump'),$archiveBytes[0..([int]($archiveBytes.Length/2)-1)])
    $corrupt = New-VerificationProject 'Truncated archive' "acceptance-$runSuffix/truncated.dump" @()
    Assert-Verification '07-truncated-archive' (Wait-Verification (Start-Verification $corrupt).id) 'failed'
    $cancelProject = New-VerificationProject 'Cancellation' $pattern @(@{name='Cancelable query';sql='SELECT pg_sleep(25) IS NULL'})
    $cancelJob = Start-Verification $cancelProject
    $queryStarted = $false
    for ($attempt=0;$attempt -lt 100;$attempt++) {
        $current = Invoke-RestMethod "$base/api/jobs/$($cancelJob.id)"
        if ($current.status -notin @('queued','running')) { throw 'Cancellation target finished unexpectedly.' }
        if ($current.status -eq 'running') {
            $activity = & docker exec "restivra-$($cancelJob.id)" psql -X -qAt -U postgres -d verification -c "SELECT count(*) FROM pg_stat_activity WHERE usename='restivra_verifier' AND state='active' AND query LIKE '%pg_sleep%'" 2>$null
            if ($LASTEXITCODE -eq 0 -and $activity -eq '1') { $queryStarted=$true;break }
        }
        Start-Sleep -Milliseconds 300
    }
    if (-not $queryStarted) { throw 'Could not observe the running verification query before cancellation.' }
    Invoke-RestMethod "$base/api/jobs/$($cancelJob.id)/cancel" -Method Post -Headers $headers -ContentType 'application/json' -Body '{}' | Out-Null
    Assert-Verification '08-cancel-running-query' (Wait-Verification $cancelJob.id) 'cancelled'
    $evidence = [ordered]@{source=$metrics;source_orders_after_data_loss=[int]$sourceOrders;archive_bytes=(Get-Item -LiteralPath $archive).Length;archive_sha256=(Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant();scenarios=$results;synthetic_data=$true;mock_database=$false;created_at=[DateTime]::UtcNow.ToString('o')}
    $evidence | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $evidenceDir 'summary.json') -Encoding utf8
    Write-Host "Acceptance completed: $($results.Count) scenarios. Evidence: $evidenceDir"
}
finally {
    if ($sourceCreated) {
        & docker rm -f -v $source | Out-Null
        if ($LASTEXITCODE -ne 0) { Write-Warning "Source cleanup failed: $source" }
    }
    # Only the healthy project remains in the default view. Archived projects
    # retain their real run records and reports; no run history is deleted.
    foreach ($project in $createdProjects) {
        if ($project.name -like '*/ Commerce restore') { continue }
        try { Invoke-RestMethod "$base/api/projects/$($project.id)" -Method Delete -Headers $headers -ContentType 'application/json' | Out-Null } catch { Write-Warning 'A lab project could not be archived.' }
    }
    Pop-Location
}
