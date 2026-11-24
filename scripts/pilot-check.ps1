#Requires -Version 7.2
# Read-only prerequisite check. Does not print credentials or project data.
param(
    [string]$Base,
    [string]$EnvFile = (Join-Path (Split-Path -Parent $PSScriptRoot) '.env')
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'api-session.ps1')
. (Join-Path $PSScriptRoot 'release-ops.ps1')
$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
    $Base=Resolve-RestivraBase -Base $Base -EnvFile $EnvFile
    $uri = [uri]$Base
    if (-not $uri.IsLoopback -or $uri.Scheme -ne 'http') { throw 'The desktop pilot requires loopback HTTP.' }
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) { throw 'Install Docker Desktop and start its Linux engine first.' }
    $engine = & docker info --format '{{.OSType}}'
    if ($LASTEXITCODE -ne 0 -or $engine.Trim() -ne 'linux') { throw 'Docker must be running with Linux containers.' }
    & docker compose version --short | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Docker Compose is required.' }
    & docker image inspect postgres:17-bookworm --format '{{.Id}}' | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Run: docker pull postgres:17-bookworm' }
    Release-LocalEngine
    foreach($service in @('api','worker','db')){
        $container=Release-Service $service
        if(-not $container.running -or $container.paused){throw 'Pilot services must be running and unpaused before the prerequisite check.'}
    }
    $api=Release-Service api
    $ports=(Release-Docker @('inspect',$api.id,'--format','{{json .NetworkSettings.Ports}}'))|ConvertFrom-Json
    $bindings=@($ports.'7474/tcp')
    if(-not @($bindings|Where-Object{$_.HostPort -eq "$($uri.Port)" -and $_.HostIp -in @('127.0.0.1','::1')}).Count -or
       @($bindings|Where-Object{$_.HostIp -notin @('127.0.0.1','::1')}).Count){
        throw 'The selected API origin does not match this installation''s loopback Docker port. Check RESTIVRA_PORT and terminal overrides.'
    }
    $connection = New-RestivraSession -Base $Base -EnvFile $EnvFile
    $health = Invoke-RestMethod "$Base/api/health" -WebSession $connection.WebSession -TimeoutSec 15 -MaximumRedirection 0
    if (-not $health.authentication_enabled) { throw 'Pilot sign-in is disabled. Follow the existing-installation instructions in docs/PILOT-RUNBOOK.md; do not replace an initialized database password.' }
    if (-not $health.worker_online) { throw 'Worker is offline. Check docker compose logs --tail=100 worker.' }
    & docker compose exec -T api restivra healthcheck
    if ($LASTEXITCODE -ne 0) { throw 'API/database healthcheck failed.' }
    & docker compose exec -T worker restivra doctor --role worker
    if ($LASTEXITCODE -ne 0) { throw 'Worker prerequisites failed.' }
    Write-Host 'Pilot prerequisites passed. This check does not run a restore.'
} finally { Pop-Location }
