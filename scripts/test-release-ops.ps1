#Requires -Version 7.2
# Offline safety contracts; integration evidence is recorded separately.
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'release-ops.ps1')
$originalRoot=$script:ReleaseRoot
$scratch=Join-Path $originalRoot ('work/release-unit-'+[guid]::NewGuid().ToString('N'))
$null=New-Item -ItemType Directory -Force $scratch
$script:ReleaseRoot=$scratch
function Expect-Rejection([scriptblock]$Action){$rejected=$false;try{&$Action}catch{$rejected=$true};if(-not $rejected){throw 'An unsafe operation was accepted.'}}
try{
    $m=[pscustomobject]@{format='restivra.release.v1';version='v0.1.0-pilot.3';revision=('a'*40);image=('ghcr.io/lazypanic999/restivra@sha256:'+('b'*64));binary_sha256=('c'*64);platform='linux/amd64';metadata_contract=1}
    Release-ValidateManifest $m $m.version
    foreach($field in @('format','version','revision','image','binary_sha256','platform','metadata_contract')){
        $bad=$m|ConvertTo-Json|ConvertFrom-Json
        $bad.$field='invalid'
        Expect-Rejection { Release-ValidateManifest $bad $m.version }
    }
    Expect-Rejection {Release-ReadManifest '../latest'}
    $path=Join-Path $scratch '.env'
    $secret='POSTGRES_PASSWORD=synthetic-not-a-secret'+"`n"+'RESTIVRA_AUTH_TOKEN=synthetic-token'+"`n"+'CUSTOM_SETTING="a b"'+"`n"
    [IO.File]::WriteAllText($path,$secret)
    Release-SetImage $m.image
    Release-SetImage $m.image
    $saved=[IO.File]::ReadAllText($path)
    if(-not $saved.StartsWith($secret) -or [regex]::Matches($saved,'(?m)^RESTIVRA_IMAGE=').Count -ne 1){throw 'Image selection changed private settings or duplicated a key.'}
    [IO.File]::AppendAllText($path,"RESTIVRA_IMAGE=duplicate`n")
    $before=(Get-FileHash $path).Hash
    Expect-Rejection {Release-SetImage $m.image}
    if((Get-FileHash $path).Hash -ne $before){throw 'Rejected update modified .env.'}
    $lock=Release-Lock
    try{Expect-Rejection {$other=Release-Lock;$other.Dispose()}}finally{$lock.Dispose()}
    $lock=Release-Lock;$lock.Dispose()
    Write-Host 'Release manifest validation, credential preservation, duplicate-key rejection and exclusive lock passed.'
}finally{$script:ReleaseRoot=$originalRoot}
