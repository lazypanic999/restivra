#Requires -Version 7.2
# Always creates a fresh synthetic installation; never accepts an existing root.
param(
    [Parameter(Mandatory)][string]$ApplicationImage,
    [ValidateRange(1024,65535)][int]$Port=27479
)
$ErrorActionPreference='Stop'
$sourceRoot=Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
. (Join-Path $sourceRoot 'scripts/release-ops.ps1')
Release-LocalEngine
$imageID=Release-ImageID $ApplicationImage
$suffix=[guid]::NewGuid().ToString('N')
$testRoot=Join-Path $sourceRoot "work/browser-$suffix"
$null=New-Item -ItemType Directory -Force $testRoot
Copy-Item (Join-Path $sourceRoot 'compose.yaml') $testRoot
Copy-Item (Join-Path $sourceRoot 'scripts') (Join-Path $testRoot 'scripts') -Recurse
$project="restivra-browser-$($suffix.Substring(0,12))"
$envPath=Join-Path $testRoot '.env'
$artifacts=Join-Path $testRoot 'artifacts'
$null=New-Item -ItemType Directory $artifacts
$browserImage="restivra:browser-qa-$suffix"
Write-Host 'Building the pinned Linux browser test runner...'
Release-Docker @('build','--tag',$browserImage,$PSScriptRoot) 900 | Out-Null
$browserID=Release-ImageID $browserImage
$previousRoot=$script:ReleaseRoot
$previousProject=$env:COMPOSE_PROJECT_NAME
$previousPort=$env:RESTIVRA_PORT
$env:COMPOSE_PROJECT_NAME=$project
$env:RESTIVRA_PORT="$Port"
$script:ReleaseRoot=$testRoot
try {
    & (Join-Path $testRoot 'scripts/setup.ps1')
    Release-SetImage $imageID
    Release-Docker @('compose','up','--no-build','--pull','never','-d','--wait','--wait-timeout','120') 150|Out-Null
    # A wrong explicit port must be refused before authentication or mutations.
    $wrongPort=if($Port -eq 65535){65534}else{$Port+1}
    $refused=$false
    try{& (Join-Path $testRoot 'scripts/pilot-check.ps1') -Base "http://127.0.0.1:$wrongPort"}
    catch{if($_.Exception.Message -match 'origin does not match'){$refused=$true}else{throw}}
    if(-not $refused){throw 'A different installation port was not refused.'}
    # No -Base: prove the saved/selected port is used instead of hardcoded 7474.
    & (Join-Path $testRoot 'scripts/pilot-smoke.ps1')
    # Only the synthetic completed failure is marked undelivered. No webhook
    # URL is configured and this terminal state never sends a notification.
    Release-SQL "UPDATE jobs SET notification_status='failed',notification_attempts=3 WHERE id=(SELECT id FROM jobs WHERE status='failed' ORDER BY queued_at LIMIT 1)"|Out-Null
    # Use the freshly generated smoke project's valid config/archive as a
    # template; no real project, SQL, storage profile or metadata is copied.
    Release-SQL "INSERT INTO projects(id,name,backup_pattern,config,timezone,created_at,archived) SELECT md5('browser-$suffix-'||n::text),'BROWSER_PRIVATE_CANARY '||CASE WHEN n=610 THEN 'BROWSER_OLDEST_TARGET' ELSE lpad(n::text,4,'0') END,p.backup_pattern,p.config,'UTC',now()-n*interval '1 minute',n>610 FROM generate_series(1,620) n CROSS JOIN (SELECT backup_pattern,config FROM projects ORDER BY created_at LIMIT 1) p"|Out-Null
    $tokenLine=Get-Content $envPath|Where-Object{$_ -match '^RESTIVRA_AUTH_TOKEN='}|Select-Object -First 1
    $tokenPath=Join-Path $testRoot 'private/browser-token'
    [IO.File]::WriteAllText($tokenPath,$tokenLine.Substring('RESTIVRA_AUTH_TOKEN='.Length),[Text.UTF8Encoding]::new($false))
    $apiID=(Release-Service api).id
    # The Linux browser shares only this test API's network namespace. Mount
    # only its generated UI token and output directory, never the Docker socket.
    Release-Docker @('run','--rm','--init','--network',"container:$apiID",'--memory','1536m','--cpus','2','--pids-limit','256','--shm-size','256m','--mount',"type=bind,source=$tokenPath,target=/run/restivra-test-token,readonly",'--mount',"type=bind,source=$artifacts,target=/artifacts",$browserID) 240|Out-Null
    $proof=Get-Content (Join-Path $artifacts 'browser-summary.json') -Raw|ConvertFrom-Json
    if($proof.status -ne 'passed'){throw 'Browser checks did not pass. Inspect the private artifacts.'}
    Write-Host 'Browser and pilot endpoint checks passed. See artifacts/browser-summary.json for the workflow assertions.'
} finally {
    try {Release-Docker @('compose','down') 90|Out-Null}catch{Write-Warning 'Synthetic browser project cleanup needs review.'}
    $script:ReleaseRoot=$previousRoot
    $env:COMPOSE_PROJECT_NAME=$previousProject
    $env:RESTIVRA_PORT=$previousPort
    Write-Host "Browser evidence and private test configuration retained under $testRoot"
}
