#Requires -Version 7.2
# Shared authenticated API connection for local acceptance/demo scripts.
function New-RestivraSession {
    param([string]$Base = 'http://127.0.0.1:7474', [string]$EnvFile = (Join-Path (Split-Path -Parent $PSScriptRoot) '.env'))
    $uri = [uri]$Base
    if ($uri.Scheme -ne 'https' -and -not ($uri.Scheme -eq 'http' -and $uri.IsLoopback)) { throw 'API credentials require HTTPS or loopback HTTP.' }
    $webSession = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
    $auth = Invoke-RestMethod "$Base/api/auth" -WebSession $webSession -MaximumRedirection 0
    $headers = @{'X-CSRF-Token'=$auth.csrf_token}
    if ($auth.enabled) {
        $access = $env:RESTIVRA_AUTH_TOKEN
        if (-not $access -and (Test-Path -LiteralPath $EnvFile)) {
            $line = Get-Content -LiteralPath $EnvFile | Where-Object { $_ -match '^RESTIVRA_AUTH_TOKEN=' } | Select-Object -First 1
            if ($line) { $access = $line.Substring('RESTIVRA_AUTH_TOKEN='.Length) }
        }
        if (-not $access) { throw 'RESTIVRA_AUTH_TOKEN is required; set it in the environment or private .env file.' }
        try { Invoke-RestMethod "$Base/api/auth/login" -Method Post -Headers $headers -WebSession $webSession -MaximumRedirection 0 -ContentType 'application/json' -Body (@{token=$access}|ConvertTo-Json -Compress) | Out-Null }
        finally { $access=$null }
    }
    $session = Invoke-RestMethod "$Base/api/session" -WebSession $webSession -MaximumRedirection 0
    return @{ Session=$session; Headers=@{'X-CSRF-Token'=$session.csrf_token}; WebSession=$webSession }
}
