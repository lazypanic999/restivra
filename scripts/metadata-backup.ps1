#Requires -Version 7.2
param([Parameter(Mandatory)][string]$OutputPath)
$ErrorActionPreference = 'Stop'
$projectDir = Split-Path -Parent $PSScriptRoot
$output = [IO.Path]::GetFullPath($OutputPath)
$partial = "$output.partial"
if (Test-Path -LiteralPath $output) { throw 'Output already exists.' }
$stream = [IO.File]::Open($partial,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
$process = [Diagnostics.Process]::new()
try {
    $process.StartInfo.FileName='docker'
    $process.StartInfo.WorkingDirectory=$projectDir
    $process.StartInfo.UseShellExecute=$false
    $process.StartInfo.CreateNoWindow=$true
    $process.StartInfo.RedirectStandardOutput=$true
    $process.StartInfo.RedirectStandardError=$true
    foreach($argument in @('compose','exec','-T','db','pg_dump','-U','restivra','-d','restivra','-Fc')) { $process.StartInfo.ArgumentList.Add($argument) }
    if(-not $process.Start()){throw 'Could not start metadata export.'}
    $diagnostic=$process.StandardError.ReadToEndAsync()
    $process.StandardOutput.BaseStream.CopyTo($stream)
    $process.WaitForExit()
    $null=$diagnostic.GetAwaiter().GetResult()
    if($process.ExitCode -ne 0){throw 'Metadata export failed. The .partial file is not a completed archive.'}
    $stream.Flush($true)
}finally{$stream.Dispose();$process.Dispose()}
# File.Move without overwrite refuses a destination created during the export.
[IO.File]::Move($partial,$output)
Write-Host 'Metadata archive created. Keep it private; test its restore separately.'
