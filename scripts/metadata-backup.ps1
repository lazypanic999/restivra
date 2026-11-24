#Requires -Version 7.2
param(
    [Parameter(Mandatory)][string]$OutputPath,
    [ValidateRange(1,3600)][int]$TimeoutSeconds = 180,
    [ValidateRange(1,10240)][int]$MaxArchiveMB = 512
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'native-command.ps1')
. (Join-Path $PSScriptRoot 'private-io.ps1')
$projectDir = Split-Path -Parent $PSScriptRoot
$output = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutputPath)
$partial = "$output.partial"
if (Test-Path -LiteralPath $output) { throw 'Output already exists.' }
$stream = New-RestivraPrivateFile $partial
try {
    $result = Invoke-RestivraNative -FilePath docker -WorkingDirectory $projectDir -Arguments @('compose','exec','-T','db','pg_dump','-U','restivra','-d','restivra','-Fc',"--lock-wait-timeout=${TimeoutSeconds}s") -TimeoutSeconds $TimeoutSeconds -OutputStream $stream -OutputByteLimit ([long]$MaxArchiveMB * 1MB)
    if($result.ExitCode -ne 0){throw 'Metadata export failed.'}
    $stream.Flush($true)
} catch {
    throw 'Metadata export failed or exceeded its time/size limit. The .partial file is not a completed archive.'
} finally { $stream.Dispose() }
# File.Move without overwrite refuses a destination created during the export.
[IO.File]::Move($partial,$output)
Write-Host 'Metadata archive created. Keep it private; test its restore separately.'
