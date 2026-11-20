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
$binary=[IO.MemoryStream]::new()
try {
 $bytes=New-Object byte[] 1048576
 for($i=0;$i -lt $bytes.Length;$i++){$bytes[$i]=$i % 256}
 $expected=[Convert]::ToHexString([Security.Cryptography.SHA256]::HashData($bytes))
 $result=Invoke-RestivraNative -FilePath $executable -Arguments @('-NoProfile','-NonInteractive','-Command','$b=New-Object byte[] 1048576;for($i=0;$i -lt $b.Length;$i++){$b[$i]=$i % 256};[Console]::Error.Write("private-binary-sentinel" * 20000);[Console]::OpenStandardOutput().Write($b,0,$b.Length)') -OutputStream $binary -OutputByteLimit 1048576
 if($result.ExitCode -ne 0 -or $result.Output -or $binary.Length -ne $bytes.Length -or [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData($binary.ToArray())) -ne $expected){throw 'Binary output was altered or retained as text'}
} finally {$binary.Dispose()}
$limited=[IO.MemoryStream]::new()
try {
 $watch.Restart()
 $failed=$false
 try {Invoke-RestivraNative -FilePath $executable -Arguments @('-NoProfile','-NonInteractive','-Command','[Console]::OpenStandardOutput().Write([byte[]]::new(8192),0,8192);Start-Sleep -Seconds 15') -OutputStream $limited -OutputByteLimit 1024|Out-Null}
 catch {$failed=$_.Exception.Message -eq 'Native command or stream failed.'}
 if(-not $failed -or $limited.Length -gt 1024 -or $watch.Elapsed.TotalSeconds -gt 7){throw 'Binary output overflow did not stop promptly within its byte limit'}
} finally {$limited.Dispose()}
$partial=[IO.MemoryStream]::new()
try {
 $watch.Restart()
 $failed=$false
 try {Invoke-RestivraNative -FilePath $executable -Arguments @('-NoProfile','-NonInteractive','-Command','[Console]::OpenStandardOutput().WriteByte(80);Start-Sleep -Seconds 15') -OutputStream $partial -TimeoutSeconds 1|Out-Null}
 catch {$failed=$_.Exception.Message -eq 'Native command exceeded its time limit.'}
 if(-not $failed -or $watch.Elapsed.TotalSeconds -gt 7){throw 'Binary output did not respect the process timeout'}
} finally {$partial.Dispose()}
$unwritable=[IO.MemoryStream]::new([byte[]]::new(0),$false)
try {
 $watch.Restart()
 $failed=$false
 try {Invoke-RestivraNative -FilePath $executable -Arguments @('-NoProfile','-NonInteractive','-Command','[Console]::OpenStandardOutput().WriteByte(80);Start-Sleep -Seconds 15') -OutputStream $unwritable|Out-Null}
 catch {$failed=$_.Exception.Message -eq 'Native command or stream failed.'}
 if(-not $failed -or $watch.Elapsed.TotalSeconds -gt 7){throw 'An unwritable binary destination did not stop promptly with a sanitized error'}
} finally {$unwritable.Dispose()}
Write-Host 'Native process checks passed: capped text, private stderr, bounded timeout, sanitized errors, exact binary streaming and prompt size/write-error rejection.'
