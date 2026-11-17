#Requires -Version 7.2
# Creates only synthetic data, runs the real CLI, then removes the fixture container.
$ErrorActionPreference = 'Stop'
$projectDir = Split-Path -Parent $PSScriptRoot
$fixtureName = 'restivra-demo-' + [guid]::NewGuid().ToString('N')
$workDir = Join-Path $projectDir 'work'
$backupPath = Join-Path $workDir 'demo.dump'
$reportPath = Join-Path $workDir ('demo-report-' + [guid]::NewGuid().ToString('N') + '.json')
New-Item -ItemType Directory -Path $workDir -Force | Out-Null

Push-Location $projectDir
try {
    docker run --detach --name $fixtureName --network none --memory 512m --cpus 1 --pids-limit 128 --env POSTGRES_HOST_AUTH_METHOD=trust postgres:17-bookworm | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Fixture could not start. Check Docker and pull postgres:17-bookworm.' }
    $ready = $false
    for ($attempt = 0; $attempt -lt 60; $attempt++) {
        docker exec $fixtureName sh -c 'test "$(cat /proc/1/comm)" = postgres && pg_isready -q -U postgres' 2>$null
        if ($LASTEXITCODE -eq 0) { $ready = $true; break }
        Start-Sleep -Milliseconds 500
    }
    if (-not $ready) { throw 'Fixture readiness timed out.' }
    docker exec $fixtureName psql -X -U postgres -v ON_ERROR_STOP=1 -c "CREATE TABLE customers(id integer PRIMARY KEY, name text NOT NULL); INSERT INTO customers VALUES (1, 'Ada'), (2, 'Deniz');" | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Fixture schema failed.' }
    docker exec $fixtureName pg_dump -U postgres -Fc -f /tmp/demo.dump postgres
    if ($LASTEXITCODE -ne 0) { throw 'Fixture dump failed.' }
    docker cp "${fixtureName}:/tmp/demo.dump" $backupPath
    if ($LASTEXITCODE -ne 0) { throw 'Fixture copy failed.' }
    go run ./cmd/restivra verify --backup $backupPath --config config.smoke.json --report $reportPath
    if ($LASTEXITCODE -ne 0) { throw 'Verification failed. Read the JSON result above.' }
    Write-Host "Synthetic demo passed. Report: $reportPath"
}
finally {
    docker rm --force --volumes $fixtureName | Out-Null
    if ($LASTEXITCODE -ne 0) { Write-Warning "Fixture cleanup failed: $fixtureName" }
    Pop-Location
}
