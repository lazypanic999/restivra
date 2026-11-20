#Requires -Version 7.2
# Shared release operations. Native stderr/configuration may contain credentials:
# retain only bounded stdout and never print Compose's expanded configuration.
. (Join-Path $PSScriptRoot 'native-command.ps1')
$script:ReleaseRoot = [IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
function Release-Docker([string[]]$Arguments, [int]$Seconds=60) {
    $r = Invoke-RestivraNative -FilePath docker -WorkingDirectory $script:ReleaseRoot -Arguments $Arguments -TimeoutSeconds $Seconds
    if ($r.ExitCode -ne 0 -or $r.Truncated) { throw "Docker operation failed: $($Arguments[0]). No credentials were logged." }
    $r.Output.Trim()
}
function Release-LocalEngine {
    if ($env:COMPOSE_FILE -or $env:RESTIVRA_IMAGE) { throw 'Unset COMPOSE_FILE and RESTIVRA_IMAGE; release operations use this directory and its .env.' }
    $context = if ($env:DOCKER_CONTEXT) { $env:DOCKER_CONTEXT } else { Release-Docker @('context','show') }
    $endpoints = @(Release-Docker @('context','inspect',$context,'--format','{{.Endpoints.docker.Host}}'))
    if ($env:DOCKER_HOST) { $endpoints += $env:DOCKER_HOST }
    foreach ($endpoint in $endpoints) { if ($endpoint -notmatch '^unix:///' -and $endpoint -notmatch '^npipe:////\./pipe/') { throw 'A local Docker engine is required.' } }
    if ((Release-Docker @('info','--format','{{.OSType}}/{{.Architecture}}')) -notin @('linux/x86_64','linux/amd64')) { throw 'This pilot release supports Linux/amd64 Docker engines only.' }
}
function Release-Lock {
    $null = New-Item -ItemType Directory -Force (Join-Path $script:ReleaseRoot 'private')
    try { [IO.File]::Open((Join-Path $script:ReleaseRoot 'private/release-operation.lock'),[IO.FileMode]::OpenOrCreate,[IO.FileAccess]::ReadWrite,[IO.FileShare]::None) }
    catch { throw 'Another install/update operation holds the installation lock.' }
}
function Release-ReadManifest([string]$Version) {
    if ($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+(-[a-z0-9]+([.-][a-z0-9]+)*)?$') { throw 'Specify an explicit release tag, for example v0.1.0-pilot.3.' }
    $url = "https://github.com/lazypanic999/restivra/releases/download/$Version/restivra-release.json"
    $client = [Net.Http.HttpClient]::new()
    $client.Timeout = [TimeSpan]::FromSeconds(45)
    try {
        $client.MaxResponseContentBufferSize = 65536
        $raw = $client.GetStringAsync($url).GetAwaiter().GetResult()
        $m = $raw | ConvertFrom-Json -ErrorAction Stop
    } catch { throw 'Official release manifest unavailable. Check the version and internet connection.' }
    finally { $client.Dispose() }
    Release-ValidateManifest $m $Version
    $m
}
function Release-ValidateManifest($m, [string]$Version) {
    if ($m.format -ne 'restivra.release.v1' -or $m.version -cne $Version -or
        $m.version -cnotmatch '^v[0-9]+\.[0-9]+\.[0-9]+(-[a-z0-9]+([.-][a-z0-9]+)*)?$' -or
        $m.image -cnotmatch '^ghcr\.io/lazypanic999/restivra@sha256:[a-f0-9]{64}$' -or
        $m.revision -cnotmatch '^[a-f0-9]{40}$' -or $m.binary_sha256 -cnotmatch '^[a-f0-9]{64}$' -or
        $m.platform -ne 'linux/amd64' -or $m.metadata_contract -ne 1) { throw 'Unsupported or invalid release manifest; no services changed.' }
}
function Release-ImageID([string]$Image) { Release-Docker @('image','inspect',$Image,'--format','{{.Id}}') }
function Release-BinarySHA([string]$Image) {
    $line = Release-Docker @('run','--rm','--network','none','--read-only','--cap-drop','ALL','--security-opt','no-new-privileges:true','--entrypoint','sha256sum',$Image,'/usr/local/bin/restivra')
    if ($line -notmatch '^([a-f0-9]{64})\s') { throw 'Cannot verify the application binary.' }
    $Matches[1]
}
function Release-VerifyImage($Manifest, [switch]$Pull) {
    Release-ValidateManifest $Manifest $Manifest.version
    if ($Pull) { Write-Host "Downloading $($Manifest.version)..."; Release-Docker @('pull',$Manifest.image) 600 | Out-Null }
    $labels = (Release-Docker @('image','inspect',$Manifest.image,'--format','{{json .Config.Labels}}')) | ConvertFrom-Json
    if ($labels.'org.opencontainers.image.source' -ne 'https://github.com/lazypanic999/restivra' -or
        $labels.'org.opencontainers.image.version' -cne $Manifest.version -or
        $labels.'org.opencontainers.image.revision' -cne $Manifest.revision -or $labels.'io.restivra.metadata-contract' -ne '1' -or
        (Release-BinarySHA $Manifest.image) -cne $Manifest.binary_sha256) { throw 'Downloaded image does not match the official manifest.' }
    Release-ImageID $Manifest.image
}
function Release-JSON([string]$Path, $Value) {
    $temp = "$Path.$([guid]::NewGuid().ToString('N')).tmp"
    [IO.File]::WriteAllText($temp,($Value | ConvertTo-Json -Depth 20)+"`n",[Text.UTF8Encoding]::new($false))
    [IO.File]::Move($temp,$Path,$true)
}
function Release-SetImage([string]$Image) {
    $path = Join-Path $script:ReleaseRoot '.env'
    $raw = [IO.File]::ReadAllText($path)
    $pattern = '(?m)^RESTIVRA_IMAGE=[^\r\n]*'
    if ([regex]::Matches($raw,$pattern).Count -gt 1) { throw 'Duplicate RESTIVRA_IMAGE settings in .env.' }
    $raw = if ([regex]::IsMatch($raw,$pattern)) { [regex]::Replace($raw,$pattern,"RESTIVRA_IMAGE=$Image") } else { $raw.TrimEnd()+"`nRESTIVRA_IMAGE=$Image`n" }
    $temp = "$path.$([guid]::NewGuid().ToString('N')).tmp"
    [IO.File]::WriteAllText($temp,$raw,[Text.UTF8Encoding]::new($false))
    [IO.File]::Move($temp,$path,$true)
}
function Release-Service([string]$Service) {
    $ids = @( (Release-Docker @('compose','ps','--all','--quiet',$Service)) -split '\r?\n' | Where-Object { $_ })
    if ($ids.Count -ne 1) { throw "Expected one existing $Service container for this installation." }
    $id = $ids[0]
    $owner = Release-Docker @('inspect',$id,'--format','{{index .Config.Labels "com.docker.compose.project.working_dir"}}')
    if ([IO.Path]::GetFullPath($owner) -ne $script:ReleaseRoot) { throw 'Compose project belongs to a different directory. Choose a unique COMPOSE_PROJECT_NAME.' }
    [pscustomobject]@{id=$id;image=(Release-Docker @('inspect',$id,'--format','{{.Image}}'));running=(Release-Docker @('inspect',$id,'--format','{{.State.Running}}')) -eq 'true'}
}
function Release-SQL([string]$SQL) { Release-Docker @('compose','exec','-T','db','psql','-X','-At','-U','restivra','-d','restivra','-v','ON_ERROR_STOP=1','-c',$SQL) }
function Release-Idle {
    if ((Release-SQL "SELECT count(*) FROM jobs WHERE status IN ('queued','running')") -ne '0') { throw 'Queued/running jobs exist. Wait for an idle window and try again.' }
}
function Release-Start([string]$Image, [int]$WaitSeconds=120) {
    $expectedImageID=Release-ImageID $Image
    $env:RESTIVRA_IMAGE = $Image
    try {
        Release-Docker @('compose','up','--no-build','--no-deps','--pull','never','-d','--wait','--wait-timeout',"$WaitSeconds",'api') ($WaitSeconds+30) | Out-Null
        if((Release-Service 'api').image -ne $expectedImageID){throw 'API did not start the requested image; inspect Compose overrides.'}
        Release-Docker @('compose','run','--rm','--no-deps','worker','doctor','--role','worker') 60 | Out-Null
        $config=(Release-Docker @('compose','config','--format','json'))|ConvertFrom-Json
        $instance=$config.services.worker.environment.RESTIVRA_INSTANCE.Replace("'","''")
        $started=Release-SQL "SELECT clock_timestamp()::text"
        Release-Docker @('compose','up','--no-build','--no-deps','--pull','never','-d','worker') | Out-Null
        # A running process alone is insufficient: require a fresh worker heartbeat.
        for ($n=0; $n -lt 30; $n++) {
            $worker = Release-Service 'worker'
            if (-not $worker.running -or $worker.image -ne $expectedImageID) { throw 'Worker did not remain running on the requested image.' }
            $online = Release-SQL "SELECT count(*) FROM workers WHERE instance='$instance' AND heartbeat_at > '$started'::timestamptz AND heartbeat_at > now() - interval '15 seconds'"
            if ([int]$online -gt 0) { return }
            Start-Sleep -Seconds 1
        }
        throw 'Worker heartbeat did not appear.'
    } finally { Remove-Item Env:RESTIVRA_IMAGE -ErrorAction SilentlyContinue }
}
