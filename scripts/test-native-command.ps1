#Requires -Version 7.2
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'native-command.ps1')
$executable=(Get-Process -Id $PID).Path
$large=Invoke-RestivraNative -FilePath $executable -Arguments @('-NoProfile','-NonInteractive','-Command','[Console]::Error.Write("private-native-sentinel" * 20000); [Console]::Out.Write("x" * 200000)') -OutputLimit 128
if($large.ExitCode -ne 0 -or -not $large.Truncated -or $large.Output.Length -ne 128 -or $large.Output.Contains('sentinel')){throw 'Native output cap or stderr privacy failed'}
$watch=[Diagnostics.Stopwatch]::StartNew()
$timedOut=$false
try {Invoke-RestivraNative -FilePath $executable -Arguments @('-NoProfile','-NonInteractive','-Command','Start-Sleep -Seconds 15') -TimeoutSeconds 1|Out-Null}
catch {$timedOut=$_.Exception.Message -eq 'Native command exceeded its time limit.'}
if(-not $timedOut -or $watch.Elapsed.TotalSeconds -gt 7){throw 'Native process timeout was not bounded'}
$sanitized=$false
try {Invoke-RestivraNative -FilePath '__restivra_missing_native_executable__'|Out-Null}
catch {$sanitized=$_.Exception.Message -eq 'Native command or stream failed.'}
if(-not $sanitized){throw 'Native start failure was not sanitized'}
Write-Host 'Native process checks passed: capped output, discarded private stderr, bounded timeout and sanitized start failure.'
