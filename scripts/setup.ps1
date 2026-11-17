$ErrorActionPreference = 'Stop'
$projectDir = Split-Path -Parent $PSScriptRoot
$envPath = Join-Path $projectDir '.env'
if (-not (Test-Path -LiteralPath $envPath)) {
    $random = [byte[]]::new(32)
    [Security.Cryptography.RandomNumberGenerator]::Fill($random)
    $password = [Convert]::ToHexString($random).ToLowerInvariant()
    $content = "POSTGRES_PASSWORD=$password`nRESTIVRA_INSTANCE=restivra-main`nRESTIVRA_RETENTION_DAYS=30`nRESTIVRA_WEBHOOK_URL=`n"
    [IO.File]::WriteAllText($envPath, $content, [Text.UTF8Encoding]::new($false))
    Write-Host 'Generated .env. Keep this file private.'
}
New-Item -ItemType Directory -Path (Join-Path $projectDir 'backups') -Force | Out-Null
Write-Host 'Next: docker pull postgres:17-bookworm; docker compose up --build -d'
