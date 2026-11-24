#Requires -Version 7.2
# Shared authenticated API connection for local acceptance/demo scripts.
function Resolve-RestivraBase {
    param([string]$Base, [string]$EnvFile=(Join-Path (Split-Path -Parent $PSScriptRoot) '.env'))
    if (-not $Base) {
        $port=$env:RESTIVRA_PORT
        if (-not $port -and (Test-Path -LiteralPath $EnvFile)) {
            $lines=@(Get-Content -LiteralPath $EnvFile|Where-Object{$_ -match '^RESTIVRA_PORT='})
            if ($lines.Count -gt 1) { throw 'Duplicate RESTIVRA_PORT settings in .env.' }
            if ($lines.Count -eq 1) { $port=$lines[0].Substring('RESTIVRA_PORT='.Length) }
        }
        if (-not $port) { $port='7474' }
        if ($port -notmatch '^[0-9]{1,5}$' -or [int]$port -lt 1024 -or [int]$port -gt 65535) { throw 'RESTIVRA_PORT must be an integer from 1024 to 65535.' }
        $Base="http://127.0.0.1:$([int]$port)"
    }
    $uri=$null
    if (-not [uri]::TryCreate($Base,[UriKind]::Absolute,[ref]$uri) -or
        $uri.UserInfo -or $uri.Query -or $uri.Fragment -or $uri.AbsolutePath -ne '/' -or
        ($uri.Scheme -ne 'https' -and -not ($uri.Scheme -eq 'http' -and $uri.IsLoopback))) {
        throw 'Use an HTTPS or loopback HTTP API origin without credentials, path, query or fragment.'
    }
    return $uri.GetLeftPart([UriPartial]::Authority)
}
function Invoke-RestivraSessionRequest([hashtable]$Request) {
    try { Invoke-RestMethod @Request -TimeoutSec 15 -MaximumRedirection 0 -ErrorAction Stop }
    catch { throw 'Could not open the Restivra API session. Check the selected installation, port and private access token.' }
}
function New-RestivraSession {
    param([string]$Base, [string]$EnvFile = (Join-Path (Split-Path -Parent $PSScriptRoot) '.env'))
    $Base=Resolve-RestivraBase -Base $Base -EnvFile $EnvFile
    $webSession = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
    $auth = Invoke-RestivraSessionRequest @{Uri="$Base/api/auth";WebSession=$webSession}
    $headers = @{'X-CSRF-Token'=$auth.csrf_token}
    if ($auth.enabled) {
        $access = $env:RESTIVRA_AUTH_TOKEN
        if (-not $access -and (Test-Path -LiteralPath $EnvFile)) {
            $line = Get-Content -LiteralPath $EnvFile | Where-Object { $_ -match '^RESTIVRA_AUTH_TOKEN=' } | Select-Object -First 1
            if ($line) { $access = $line.Substring('RESTIVRA_AUTH_TOKEN='.Length) }
        }
        if (-not $access) { throw 'RESTIVRA_AUTH_TOKEN is required; set it in the environment or private .env file.' }
        try { Invoke-RestivraSessionRequest @{Uri="$Base/api/auth/login";Method='Post';Headers=$headers;WebSession=$webSession;ContentType='application/json';Body=(@{token=$access}|ConvertTo-Json -Compress)} | Out-Null }
        finally { $access=$null }
    }
    $session = Invoke-RestivraSessionRequest @{Uri="$Base/api/session";WebSession=$webSession}
    return @{ Base=$Base; Session=$session; Headers=@{'X-CSRF-Token'=$session.csrf_token}; WebSession=$webSession }
}
