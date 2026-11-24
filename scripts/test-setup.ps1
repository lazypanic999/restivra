#Requires -Version 7.2
# Exercises fresh, repeated and invalid setup in disposable directories only.
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'release-ops.ps1')
Release-LocalEngine
$sourceRoot=$script:ReleaseRoot
$scratch=Join-Path $sourceRoot ('work/setup-'+[guid]::NewGuid().ToString('N'))
$null=New-Item -ItemType Directory $scratch
$oldProject=$env:COMPOSE_PROJECT_NAME;$oldPort=$env:RESTIVRA_PORT
function New-SetupRoot([string]$Name) {
    $root=Join-Path $scratch $Name
    $null=New-Item -ItemType Directory (Join-Path $root 'scripts')
    Copy-Item (Join-Path $sourceRoot 'scripts/setup.ps1') (Join-Path $root 'scripts/setup.ps1')
    Copy-Item (Join-Path $sourceRoot 'scripts/setup.sh') (Join-Path $root 'scripts/setup.sh')
    Copy-Item (Join-Path $sourceRoot 'scripts/private-io.ps1') (Join-Path $root 'scripts/private-io.ps1')
    Copy-Item (Join-Path $sourceRoot 'scripts/native-command.ps1') (Join-Path $root 'scripts/native-command.ps1')
    Copy-Item (Join-Path $sourceRoot 'compose.yaml') $root
    return $root
}
function Settings([string]$Root) {
    $values=@{}
    foreach($line in Get-Content (Join-Path $Root '.env')){
        if($line -match '^([A-Z_]+)=(.*)$'){
            if($values.ContainsKey($Matches[1])){throw 'Duplicate generated setup setting.'}
            $values[$Matches[1]]=$Matches[2]
        }
    }
    return $values
}
function Check-Selection([string]$Root,[string]$Project,[int]$Port) {
    $script:ReleaseRoot=$Root
    $config=(Release-Docker @('compose','config','--format','json'))|ConvertFrom-Json
    if($config.name -cne $Project -or $config.services.api.ports[0].published -ne "$Port"){
        throw 'Compose did not select the saved project/port after clearing terminal overrides.'
    }
}
try {
    $env:COMPOSE_PROJECT_NAME=$null;$env:RESTIVRA_PORT=$null
    $first=New-SetupRoot 'first';$second=New-SetupRoot 'second'
    & (Join-Path $first 'scripts/setup.ps1')
    & (Join-Path $second 'scripts/setup.ps1')
    $a=Settings $first;$b=Settings $second
    if(-not $IsWindows){
        if((Get-RestivraUnixMode (Join-Path $first '.env')) -ne 384 -or (Get-RestivraUnixMode (Join-Path $first 'private/sources.json')) -ne 384 -or (Get-RestivraUnixMode (Join-Path $first 'private')) -ne 448){throw 'PowerShell setup did not create owner-only private files/directories.'}
    }
    if($a.COMPOSE_PROJECT_NAME -notmatch '^restivra-[a-f0-9]{12}$' -or $a.COMPOSE_PROJECT_NAME -ceq $b.COMPOSE_PROJECT_NAME){throw 'Fresh installations did not receive separate project identities.'}
    foreach($key in @('POSTGRES_PASSWORD','RESTIVRA_AUTH_TOKEN')){
        if($a[$key] -notmatch '^[a-f0-9]{64}$' -or $a[$key] -ceq $b[$key]){throw 'Fresh installations did not receive independent credentials.'}
    }
    Check-Selection $first $a.COMPOSE_PROJECT_NAME 7474
    Check-Selection $second $b.COMPOSE_PROJECT_NAME 7474
    $custom=New-SetupRoot 'custom'
    $env:COMPOSE_PROJECT_NAME='restivra-setup-custom';$env:RESTIVRA_PORT='27486'
    & (Join-Path $custom 'scripts/setup.ps1')
    $env:COMPOSE_PROJECT_NAME=$null;$env:RESTIVRA_PORT=$null
    Check-Selection $custom 'restivra-setup-custom' 27486
    # Re-running setup must preserve custom/legacy files byte for byte even if
    # a later terminal has different environment overrides.
    foreach($root in @($first,$custom)){
        $before=(Get-FileHash (Join-Path $root '.env')).Hash
        $profiles=(Get-FileHash (Join-Path $root 'private/sources.json')).Hash
        $env:COMPOSE_PROJECT_NAME='restivra-unrelated-override';$env:RESTIVRA_PORT='27487'
        & (Join-Path $root 'scripts/setup.ps1')
        if((Get-FileHash (Join-Path $root '.env')).Hash -cne $before -or (Get-FileHash (Join-Path $root 'private/sources.json')).Hash -cne $profiles){throw 'Repeated setup changed existing private files.'}
    }
    $legacy=New-SetupRoot 'legacy'
    [IO.File]::WriteAllText((Join-Path $legacy '.env'),"POSTGRES_PASSWORD=synthetic-legacy`nRESTIVRA_AUTH_TOKEN=`n")
    $before=(Get-FileHash (Join-Path $legacy '.env')).Hash
    & (Join-Path $legacy 'scripts/setup.ps1')
    if((Get-FileHash (Join-Path $legacy '.env')).Hash -cne $before){throw 'Legacy setup identity/authentication was changed.'}
    $case=0
    foreach($bad in @(@('Uppercase','7474'),@('../foreign','7474'),@(('a'*64),'7474'),@('valid','80'),@('valid','65536'),@('valid',('7474' + "`nINJECTED=yes")))){
        $case++;$root=New-SetupRoot "invalid-$case"
        $env:COMPOSE_PROJECT_NAME=$bad[0];$env:RESTIVRA_PORT=$bad[1]
        $refused=$false
        try{& (Join-Path $root 'scripts/setup.ps1')}catch{$refused=$true}
        if(-not $refused -or (Test-Path (Join-Path $root '.env'))){throw 'Invalid setup identity/port created a configuration file.'}
    }
    $env:COMPOSE_PROJECT_NAME=$null;$env:RESTIVRA_PORT=$null
    # Validate the POSIX generator in Linux against only the disposable scratch
    # mount. No socket, existing .env, private profiles or network are exposed.
    $shell=New-SetupRoot 'shell'
    $shellUser='0:0'
    if($IsLinux){
        $setupTestUID=(& id -u).Trim();$setupTestGID=(& id -g).Trim()
        if($setupTestUID -notmatch '^\d+$' -or $setupTestGID -notmatch '^\d+$'){throw 'Cannot determine the synthetic bind mount owner.'}
        $shellUser="${setupTestUID}:${setupTestGID}"
    }
    Release-Docker @('run','--rm','--user',$shellUser,'--network','none','--cap-drop','ALL','--security-opt','no-new-privileges:true','--mount',"type=bind,source=$shell,target=/test",'--workdir','/test','--entrypoint','sh','postgres:17-bookworm','-ec','sh scripts/setup.sh; before=$(sha256sum .env private/sources.json); COMPOSE_PROJECT_NAME=ignored RESTIVRA_PORT=28000 sh scripts/setup.sh; test "$before" = "$(sha256sum .env private/sources.json)"')|Out-Null
    $s=Settings $shell
    if($s.COMPOSE_PROJECT_NAME -notmatch '^restivra-[a-f0-9]{12}$'){throw 'Shell setup did not persist its random project name.'}
    Check-Selection $shell $s.COMPOSE_PROJECT_NAME 7474
    Write-Host 'Fresh setup isolation, saved terminal-independent project/port, legacy preservation and invalid inputs passed (PowerShell and POSIX).'
} finally {
    $script:ReleaseRoot=$sourceRoot;$env:COMPOSE_PROJECT_NAME=$oldProject;$env:RESTIVRA_PORT=$oldPort
}
