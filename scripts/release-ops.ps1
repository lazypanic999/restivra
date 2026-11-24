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
function Release-RequireFreePort([ValidateRange(1024,65535)][int]$Port) {
    $listener=[Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,$Port)
    try{
        $listener.ExclusiveAddressUse=$true
        $listener.Start()
    }catch{throw 'The API port is unavailable. Choose an unused RESTIVRA_PORT before installing; existing applications were not changed.'}
    finally{$listener.Stop()}
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
function Release-AtomicText([string]$Path,[string]$Value) {
    $temp = "$Path.$([guid]::NewGuid().ToString('N')).tmp"
    $bytes=[Text.UTF8Encoding]::new($false).GetBytes($Value)
    if($bytes.Length -gt 1MB){throw 'Deployment settings exceed the supported size.'}
    $stream=[IO.File]::Open($temp,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
    try{$stream.Write($bytes,0,$bytes.Length);$stream.Flush($true)}finally{$stream.Dispose()}
    # Concurrent readers/antivirus can temporarily deny Windows rename. Retain
    # the previous complete destination and retry briefly, never truncate it.
    for($attempt=0;$attempt -lt 20;$attempt++){
        try{[IO.File]::Move($temp,$Path,$true);return}
        catch [IO.IOException]{if($attempt -eq 19){throw}}
        catch [UnauthorizedAccessException]{if($attempt -eq 19){throw}}
        Start-Sleep -Milliseconds 50
    }
}
function Release-JSON([string]$Path, $Value) {
    Release-AtomicText $Path (($Value|ConvertTo-Json -Depth 20)+"`n")
}
function Release-ReadJSON([string]$Path) {
    $stream=[IO.File]::Open($Path,[IO.FileMode]::Open,[IO.FileAccess]::Read,([IO.FileShare]::ReadWrite -bor [IO.FileShare]::Delete))
    try{
        if($stream.Length -gt 1MB){throw 'Deployment record exceeds the supported size.'}
        $reader=[IO.StreamReader]::new($stream,[Text.UTF8Encoding]::new($false),$true)
        try{return $reader.ReadToEnd()|ConvertFrom-Json -ErrorAction Stop}finally{$reader.Dispose()}
    }finally{$stream.Dispose()}
}
function Release-SetImage([string]$Image) {
    $path = Join-Path $script:ReleaseRoot '.env'
    $raw = [IO.File]::ReadAllText($path)
    $pattern = '(?m)^RESTIVRA_IMAGE=[^\r\n]*'
    if ([regex]::Matches($raw,$pattern).Count -gt 1) { throw 'Duplicate RESTIVRA_IMAGE settings in .env.' }
    $raw = if ([regex]::IsMatch($raw,$pattern)) { [regex]::Replace($raw,$pattern,"RESTIVRA_IMAGE=$Image") } else { $raw.TrimEnd()+"`nRESTIVRA_IMAGE=$Image`n" }
    Release-AtomicText $path $raw
}
function Release-Service([string]$Service, [switch]$AllowMissing) {
    $ids = @( (Release-Docker @('compose','ps','--all','--quiet',$Service)) -split '\r?\n' | Where-Object { $_ })
    $ids = @($ids | Where-Object { (Release-Docker @('inspect',$_,'--format','{{index .Config.Labels "com.docker.compose.oneoff"}}')) -eq 'False' })
    if ($AllowMissing -and $ids.Count -eq 0) { return $null }
    if ($ids.Count -ne 1) { throw "Expected one existing $Service container for this installation." }
    $id = $ids[0]
    $owner = Release-Docker @('inspect',$id,'--format','{{index .Config.Labels "com.docker.compose.project.working_dir"}}')
    if (-not (Release-OwnDirectory $owner)) { throw 'Compose project belongs to a different directory. Choose a unique COMPOSE_PROJECT_NAME.' }
    [pscustomobject]@{id=$id;image=(Release-Docker @('inspect',$id,'--format','{{.Image}}'));running=(Release-Docker @('inspect',$id,'--format','{{.State.Running}}')) -eq 'true';paused=(Release-Docker @('inspect',$id,'--format','{{.State.Paused}}')) -eq 'true'}
}
function Release-OwnDirectory([string]$Path) {
    if(-not $Path){return $false}
    try{
        $actual=[IO.Path]::TrimEndingDirectorySeparator([IO.Path]::GetFullPath($Path))
        $expected=[IO.Path]::TrimEndingDirectorySeparator($script:ReleaseRoot)
        $comparison=if($IsWindows){[StringComparison]::OrdinalIgnoreCase}else{[StringComparison]::Ordinal}
        return $actual.Equals($expected,$comparison)
    }catch{return $false}
}
function Release-StopService([string]$Service) {
    $container=Release-Service $Service -AllowMissing
    if(-not $container){return}
    if($container.paused){throw 'Unpause application containers before changing deployments.'}
    $grace=if($Service -eq 'worker'){45}else{10}
    Release-Docker @('stop','--timeout',"$grace",$container.id) ($grace+30)|Out-Null
}
function Release-NoForeignOneoffs([string]$Project,[string]$OperationID='') {
    $running=(Release-Docker @('ps','--quiet','--filter',"label=com.docker.compose.project=$Project",'--filter','label=com.docker.compose.oneoff=True')) -split '\r?\n'|Where-Object{$_}
    foreach($id in $running){
        $operation=Release-Docker @('inspect',$id,'--format','{{index .Config.Labels "io.restivra.operation"}}')
        if(-not $OperationID -or $operation -cne $OperationID){throw 'Another one-off operation is running for this Compose project. Finish it before deployment or recovery.'}
    }
}
function Release-SQL([string]$SQL) { Release-Docker @('compose','exec','-T','db','psql','-X','-At','-U','restivra','-d','restivra','-v','ON_ERROR_STOP=1','-c',$SQL) }
function Release-Idle {
    if ((Release-SQL "SELECT count(*) FROM jobs WHERE status IN ('queued','running')") -ne '0') { throw 'Queued/running jobs exist. Wait for an idle window and try again.' }
}
function Release-Start([string]$Image, [int]$WaitSeconds=120, [string]$OperationID='', [scriptblock]$OnAPIReady=$null) {
    if($OperationID -and $OperationID -cnotmatch '^[a-f0-9]{32}$'){throw 'Invalid operation identity.'}
    $expectedImageID=Release-ImageID $Image
    $env:RESTIVRA_IMAGE = $Image
    try {
        Release-Docker @('compose','up','--no-build','--no-deps','--pull','never','-d','--wait','--wait-timeout',"$WaitSeconds",'api') ($WaitSeconds+30) | Out-Null
        if((Release-Service 'api').image -ne $expectedImageID){throw 'API did not start the requested image; inspect Compose overrides.'}
        if($OnAPIReady){& $OnAPIReady}
        $doctorArgs=@('compose','run','--rm','--no-deps')
        if($OperationID){$doctorArgs+=@('--label',"io.restivra.operation=$OperationID")}
        $doctorArgs+=@('worker','doctor','--role','worker')
        Release-Docker $doctorArgs 60 | Out-Null
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

function Release-Phase($Journal,[string]$Path,[string]$Phase) {
    $Journal.phase=$Phase
    Release-JSON $Path $Journal
}

function Release-ValidateRecord($Record) {
    if(-not $Record -or $Record.image_id -cnotmatch '^sha256:[a-f0-9]{64}$'){throw 'Deployment image identity is invalid.'}
    if($Record.manifest){
        Release-ValidateManifest $Record.manifest $Record.manifest.version
        if($Record.image -cne $Record.manifest.image){throw 'Deployment manifest/image mismatch.'}
    }elseif($Record.image -cnotmatch '^restivra:rollback-[a-f0-9]{32}$' -or $Record.metadata_contract -ne 1 -or $Record.binary_sha256 -cnotmatch '^[a-f0-9]{64}$'){
        throw 'Local rollback image lacks a verified compatibility record.'
    }
}

function Release-VerifyRecord($Record) {
    Release-ValidateRecord $Record
    if($Record.manifest){
        $cached=$false
        try{$null=Release-ImageID $Record.image;$cached=$true}catch{}
        $resolved=if($cached){Release-VerifyImage $Record.manifest}else{Release-VerifyImage $Record.manifest -Pull}
    }else{
        $resolved=Release-ImageID $Record.image
        if((Release-BinarySHA $Record.image) -cne $Record.binary_sha256){throw 'Local rollback binary changed.'}
    }
    if($resolved -cne $Record.image_id){throw 'Deployment image changed or is unavailable.'}
}
