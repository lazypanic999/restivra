#Requires -Version 7.2
$ErrorActionPreference = 'Stop'
$projectDir = Split-Path -Parent $PSScriptRoot
$envPath = Join-Path $projectDir '.env'
if (-not (Test-Path -LiteralPath $envPath)) {
    $random = [byte[]]::new(32)
    [Security.Cryptography.RandomNumberGenerator]::Fill($random)
    $password = [Convert]::ToHexString($random).ToLowerInvariant()
    $instance = 'restivra-' + [guid]::NewGuid().ToString('N').Substring(0,12)
    $composeProject = if ($env:COMPOSE_PROJECT_NAME) { $env:COMPOSE_PROJECT_NAME } else { $instance }
    $port = if ($env:RESTIVRA_PORT) { $env:RESTIVRA_PORT } else { '7474' }
    if ($composeProject -cnotmatch '^[a-z0-9][a-z0-9_-]{0,62}$') { throw 'COMPOSE_PROJECT_NAME must be 1-63 lowercase letters, numbers, underscores or hyphens, starting with a letter or number.' }
    if ($port -notmatch '^[0-9]{1,5}$' -or [int]$port -lt 1024 -or [int]$port -gt 65535) { throw 'RESTIVRA_PORT must be an integer from 1024 to 65535.' }
    [Security.Cryptography.RandomNumberGenerator]::Fill($random)
    $authToken = [Convert]::ToHexString($random).ToLowerInvariant()
    $content = "POSTGRES_PASSWORD=$password`nRESTIVRA_AUTH_TOKEN=$authToken`nRESTIVRA_INSTANCE=$instance`nCOMPOSE_PROJECT_NAME=$composeProject`nRESTIVRA_PORT=$port`nRESTIVRA_RETENTION_DAYS=30`nRESTIVRA_WEBHOOK_URL=`n"
    $stream = [IO.File]::Open($envPath,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
    try { $encoded=[Text.UTF8Encoding]::new($false).GetBytes($content);$stream.Write($encoded,0,$encoded.Length) } finally { $stream.Dispose() }
    Write-Host 'Generated .env. Keep this file private.'
    Write-Host 'Sign in with RESTIVRA_AUTH_TOKEN from your private .env file.'
}
New-Item -ItemType Directory -Path (Join-Path $projectDir 'backups') -Force | Out-Null
$private = Join-Path $projectDir 'private'
New-Item -ItemType Directory -Path $private -Force | Out-Null
$sourcesPath = Join-Path $private 'sources.json'
if (-not (Test-Path -LiteralPath $sourcesPath)) {
    $stream = [IO.File]::Open($sourcesPath,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
    try { $encoded=[Text.UTF8Encoding]::new($false).GetBytes('{"s3":[]}'+"`n");$stream.Write($encoded,0,$encoded.Length) } finally { $stream.Dispose() }
}
Write-Host 'Pilot: ./scripts/install.ps1. Source build: docker compose -f compose.yaml -f compose.build.yaml up --build -d'
