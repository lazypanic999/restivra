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
    if(-not (Release-OwnDirectory $scratch) -or (Release-OwnDirectory (Join-Path $scratch 'another-installation'))){throw 'Directory ownership guard accepted a foreign root.'}
    if((Release-OwnDirectory $scratch.ToUpperInvariant()) -ne [bool]$IsWindows){throw 'Directory comparison did not respect platform case sensitivity.'}
    $m=[pscustomobject]@{format='restivra.release.v1';version='v0.1.0-pilot.3';revision=('a'*40);image=('ghcr.io/lazypanic999/restivra@sha256:'+('b'*64));binary_sha256=('c'*64);platform='linux/amd64';metadata_contract=1}
    Release-ValidateManifest $m $m.version
    $record=[pscustomobject]@{manifest=$m;image=$m.image;image_id=('sha256:'+('d'*64))}
    Release-ValidateRecord $record
    $badRecord=$record|ConvertTo-Json -Depth 8|ConvertFrom-Json
    $badRecord.image='ghcr.io/other/project@sha256:'+('e'*64)
    Expect-Rejection {Release-ValidateRecord $badRecord}
    $badRecord.image=$m.image;$badRecord.image_id='not-an-image'
    Expect-Rejection {Release-ValidateRecord $badRecord}
    $local=[pscustomobject]@{manifest=$null;image=('restivra:rollback-'+('a'*32));image_id=('sha256:'+('b'*64));metadata_contract=1;binary_sha256=('c'*64)}
    Release-ValidateRecord $local
    $local.image='restivra:local'
    Expect-Rejection {Release-ValidateRecord $local}
    if(-not ('Restivra.ReleaseTestIO' -as [type])){
        Add-Type -TypeDefinition @'
using System.IO;
using System.Threading.Tasks;
namespace Restivra {
    public static class ReleaseTestIO {
        public static async Task CloseAfter(Stream stream, int milliseconds) {
            await Task.Delay(milliseconds);
            stream.Dispose();
        }
    }
}
'@
    }
    $journalTest=Join-Path $scratch 'journal.json'
    Release-JSON $journalTest @{status='old'}
    # A genuine reader denying delete temporarily blocks atomic replacement on
    # Windows. A retry must succeed without ever truncating the old destination.
    $readerLock=[IO.File]::Open($journalTest,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::ReadWrite)
    $releaseReader=[Restivra.ReleaseTestIO]::CloseAfter($readerLock,250)
    try{Release-JSON $journalTest @{status='new'};$null=$releaseReader.GetAwaiter().GetResult()}finally{$readerLock.Dispose()}
    if((Release-ReadJSON $journalTest).status -ne 'new'){throw 'Atomic state replacement failed after a transient reader lock.'}
    if($IsWindows){
        $readerLock=[IO.File]::Open($journalTest,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::ReadWrite)
        try{Expect-Rejection {Release-JSON $journalTest @{status='incomplete'}}}finally{$readerLock.Dispose()}
        if((Release-ReadJSON $journalTest).status -ne 'new'){throw 'Failed atomic write replaced a complete destination.'}
    }
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
