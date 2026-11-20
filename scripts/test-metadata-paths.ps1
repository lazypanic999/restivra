#Requires -Version 7.2
# Set-Location intentionally differs from the native process working directory.
$ErrorActionPreference='Stop'
$root=Split-Path -Parent $PSScriptRoot
$suffix=[guid]::NewGuid().ToString('N')
$directory=Join-Path $root "work/metadata-paths-$suffix"
New-Item -ItemType Directory -Path $directory|Out-Null
$archive="state-$suffix.dump"
$report="state-$suffix.json"
$nativeDirectory=[Environment]::CurrentDirectory
Push-Location $directory
try {
 if((Get-Location).Path -eq $nativeDirectory){throw 'Path regression must use a different PowerShell location'}
 & (Join-Path $PSScriptRoot 'metadata-backup.ps1') -OutputPath $archive
 if(-not (Test-Path -LiteralPath (Join-Path $directory $archive))){throw 'Metadata export ignored the PowerShell location'}
 & (Join-Path $PSScriptRoot 'metadata-check.ps1') -Archive $archive -ReportPath $report|Out-Null
 if($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath (Join-Path $directory $report))){throw 'Metadata check ignored the PowerShell location'}
 $result=Get-Content -LiteralPath (Join-Path $directory $report) -Raw|ConvertFrom-Json
 if($result.status -ne 'passed' -or $result.cleanup -ne 'passed'){throw 'Path regression did not complete an actual metadata restore'}
 if(Test-Path -LiteralPath (Join-Path $nativeDirectory $archive)){throw 'An export escaped to the native working directory'}
 @{passed=$true;export_and_report_follow_powershell_location=$true;native_directory_unchanged=$true;actual_restore_passed=$true;verified_at=[datetime]::UtcNow.ToString('o')}|ConvertTo-Json|Set-Content (Join-Path $directory 'summary.json')
 Write-Host "PowerShell path regression passed. Evidence: $directory"
} finally {Pop-Location}
