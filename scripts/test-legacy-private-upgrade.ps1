#Requires -Version 7.2
# Real old Compose containers + old public Unix modes, then current packaging.
param([ValidateRange(1024,65535)][int]$Port=27483)
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'release-ops.ps1')
. (Join-Path $PSScriptRoot 'qa-environment.ps1')
Release-LocalEngine
$sourceRoot=$script:ReleaseRoot
$suffix=[guid]::NewGuid().ToString('N')
$root=Join-Path $sourceRoot "work/legacy-private-$suffix"
$null=New-Item -ItemType Directory $root
Copy-Item (Join-Path $sourceRoot 'scripts') (Join-Path $root 'scripts') -Recurse
$project="restivra-legacy-$($suffix.Substring(0,12))"
$qaEnvironment=Enter-RestivraQAEnvironment $project $Port
$script:ReleaseRoot=$root
$proof=[ordered]@{status='failed';synthetic_data=$true;started_at=[datetime]::UtcNow.ToString('o');checks=@()}
function Assert-Legacy($Condition,[string]$Message){if(-not $Condition){throw $Message}}
function Fingerprint {Release-SQL "SELECT json_build_object('projects',(SELECT md5(coalesce(jsonb_agg(to_jsonb(p) ORDER BY p.id)::text,'')) FROM projects p),'jobs',(SELECT md5(coalesce(jsonb_agg(to_jsonb(j) ORDER BY j.id)::text,'')) FROM jobs j),'attempts',(SELECT md5(coalesce(jsonb_agg(to_jsonb(a) ORDER BY a.job_id,a.number)::text,'')) FROM job_attempts a),'installation',(SELECT id FROM installation))::text"}
try{
    $old=Release-ReadManifest 'v0.1.0-pilot.3'
    $imageID=Release-VerifyImage $old
    # Download only immutable, official source files bound by the old manifest.
    $client=[Net.Http.HttpClient]::new();$client.Timeout=[timespan]::FromSeconds(45);$client.MaxResponseContentBufferSize=65536
    try{
        foreach($name in @('compose.yaml','scripts/setup.ps1')){
            $text=$client.GetStringAsync("https://raw.githubusercontent.com/lazypanic999/restivra/$($old.revision)/$name").GetAwaiter().GetResult()
            [IO.File]::WriteAllText((Join-Path $root $name),$text)
        }
    }finally{$client.Dispose()}
    & (Join-Path $root 'scripts/setup.ps1')
    [IO.File]::AppendAllText((Join-Path $root '.env'),"`nCOMPOSE_PROJECT_NAME=$project`nRESTIVRA_PORT=$Port`nRESTIVRA_IMAGE=$($old.image)`n")
    # Pin PostgreSQL by its existing local ID; do not move a tag used by a soak.
    $postgres=Release-ImageID 'postgres:17-bookworm'
    [IO.File]::WriteAllText((Join-Path $root 'compose.override.yaml'),"services:`n  db:`n    image: $postgres`n")
    if(-not $IsWindows){
        Assert-Legacy (((Get-RestivraUnixMode (Join-Path $root '.env')) -band 63) -ne 0) 'Fixture did not reproduce the old public configuration mode.'
    }
    Release-Docker @('compose','up','--no-build','--pull','never','-d','--wait','--wait-timeout','120','db') 150|Out-Null
    # Construct the managed-state fixture with the real old binary and topology.
    # Do not call the new installer's hardening against an old container config.
    Release-Start $old.image
    $databaseID=(Release-Service db).id
    $installation=Release-SQL 'SELECT id FROM installation'
    Release-JSON (Join-Path $root 'private/deployment.json') @{format='restivra.deployment.v1';installation=$installation;current=@{manifest=$old;image=$old.image;image_id=$imageID};previous=$null;checkpoint=$null}
    & (Join-Path $root 'scripts/pilot-smoke.ps1') -Base "http://127.0.0.1:$Port"
    $baseline=Fingerprint
    $settings=(Get-Content (Join-Path $root '.env')|Where-Object{$_ -notmatch '^RESTIVRA_IMAGE='}) -join "`n"
    $profilesHash=(Get-FileHash (Join-Path $root 'private/sources.json')).Hash
    $proof.checks+='old_binary_old_directory_mount_and_real_restore_history_established'
    # Bring current host scripts/Compose into place, leaving old containers live.
    Copy-Item (Join-Path $sourceRoot 'compose.yaml') (Join-Path $root 'compose.yaml') -Force
    Get-ChildItem (Join-Path $sourceRoot 'scripts') -File|ForEach-Object{Copy-Item -LiteralPath $_.FullName -Destination (Join-Path $root 'scripts') -Force}
    & (Join-Path $root 'scripts/update.ps1') -Version 'v0.1.0-pilot.2'
    Assert-Legacy ((Fingerprint) -ceq $baseline) 'Legacy packaging upgrade changed projects/history.'
    Assert-Legacy ((Release-Service db).id -ceq $databaseID) 'Legacy packaging upgrade replaced the database container.'
    Assert-Legacy (((Get-Content (Join-Path $root '.env')|Where-Object{$_ -notmatch '^RESTIVRA_IMAGE='}) -join "`n") -ceq $settings) 'Legacy packaging upgrade replaced credentials/settings.'
    Assert-Legacy ((Get-FileHash (Join-Path $root 'private/sources.json')).Hash -ceq $profilesHash) 'Legacy packaging upgrade rewrote source profiles.'
    if(-not $IsWindows){
        Assert-Legacy ((Get-RestivraUnixMode (Join-Path $root '.env')) -eq 384 -and (Get-RestivraUnixMode (Join-Path $root 'private/sources.json')) -eq 384 -and (Get-RestivraUnixMode (Join-Path $root 'private')) -eq 448) 'Legacy Unix configuration permissions were not restricted.'
    }
    $worker=Release-Service worker
    $mounts=(Release-Docker @('inspect',$worker.id,'--format','{{json .Mounts}}'))|ConvertFrom-Json
    Assert-Legacy (@($mounts|Where-Object{$_.Destination -eq '/run/restivra'}).Count -eq 0 -and @($mounts|Where-Object{$_.Destination -eq '/run/restivra/sources.json' -and -not $_.RW}).Count -eq 1) 'Worker still exposes the whole private directory.'
    $proof.checks+='live_legacy_topology_upgraded_with_owner_only_unix_files_and_narrow_readonly_profile_mount'
    # An atomic editor save changes the inode behind a file bind. Recreating the
    # idle worker must load the new profile without exposing its fake keys.
    . (Join-Path $root 'scripts/api-session.ps1')
    $base="http://127.0.0.1:$Port";$connection=New-RestivraSession -Base $base -EnvFile (Join-Path $root '.env')
    $originalProfiles=[IO.File]::ReadAllText((Join-Path $root 'private/sources.json'))
    $fixture=@{s3=@(@{id='atomic-reload-fixture';name='Atomic profile fixture';endpoint='https://example.com';region='us-east-1';access_key_id='SYNTHETIC_PRIVATE_KEY_CANARY';secret_access_key='SYNTHETIC_PRIVATE_SECRET_CANARY';path_style=$true})}|ConvertTo-Json -Depth 5
    Release-AtomicText (Join-Path $root 'private/sources.json') $fixture
    Release-Docker @('compose','up','--no-build','--no-deps','--pull','never','--force-recreate','-d','worker') 90|Out-Null
    $loaded=$false
    for($n=0;$n -lt 40;$n++){
        $catalog=Invoke-RestMethod "$base/api/sources" -WebSession $connection.WebSession -TimeoutSec 5
        if(@($catalog.profiles|Where-Object id -eq 'atomic-reload-fixture').Count -eq 1){$loaded=$true;break}
        Start-Sleep -Milliseconds 500
    }
    Assert-Legacy $loaded 'Recreated worker did not load the atomically replaced profile.'
    Assert-Legacy (($catalog|ConvertTo-Json -Depth 5) -notmatch 'SYNTHETIC_PRIVATE_(KEY|SECRET)_CANARY') 'Source API exposed a credential canary.'
    $proof.checks+='atomic_profile_replacement_loads_after_idle_worker_recreation_without_api_credential_exposure'
    Release-AtomicText (Join-Path $root 'private/sources.json') $originalProfiles
    & (Join-Path $root 'scripts/update.ps1') -Rollback
    Assert-Legacy ((Fingerprint) -ceq $baseline -and (Release-Service db).id -ceq $databaseID) 'Rollback after private-file migration changed metadata.'
    & (Join-Path $root 'scripts/pilot-check.ps1') -Base $base
    $proof.checks+='previous_binary_remains_healthy_after_rollback_with_hardened_private_files'
    $proof.status='passed'
}finally{
    $proof.finished_at=[datetime]::UtcNow.ToString('o')
    Release-JSON (Join-Path $root 'summary.json') $proof
    try{$null=Release-Service db -AllowMissing;Release-Docker @('compose','down') 90|Out-Null}catch{Write-Warning 'Legacy private-file fixture cleanup needs review.'}
    $script:ReleaseRoot=$sourceRoot;Restore-RestivraQAEnvironment $qaEnvironment
    Write-Host "Legacy private-file migration evidence retained under $root"
}
