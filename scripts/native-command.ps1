#Requires -Version 7.2
# Bounded native-process I/O. Never retain stderr from database operations.
if (-not ('Restivra.NativeIO' -as [type])) {
    Add-Type -TypeDefinition @'
using System.IO;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
namespace Restivra {
    public sealed class Capture {
        public string Text { get; set; }
        public bool Truncated { get; set; }
    }
    public static class NativeIO {
        public static async Task<Capture> ReadAsync(StreamReader source, int limit) {
            var text = new StringBuilder();
            var buffer = new char[4096];
            bool truncated = false;
            int count;
            while ((count = await source.ReadAsync(buffer, 0, buffer.Length)) > 0) {
                int keep = System.Math.Min(count, limit - text.Length);
                if (keep > 0) text.Append(buffer, 0, keep);
                if (keep < count) truncated = true;
            }
            return new Capture { Text = text.ToString(), Truncated = truncated };
        }
        public static async Task WriteAsync(Stream source, Stream target, CancellationToken token) {
            try { await source.CopyToAsync(target, 81920, token); }
            finally { target.Dispose(); }
        }
        public static async Task CopyOutputAsync(Stream source, Stream target, long limit, CancellationToken token) {
            var buffer = new byte[81920];
            long copied = 0;
            int count;
            while ((count = await source.ReadAsync(buffer, 0, buffer.Length, token)) > 0) {
                if (count > limit - copied) throw new IOException("Native output exceeded its byte limit.");
                await target.WriteAsync(buffer, 0, count, token);
                copied += count;
            }
        }
    }
}
'@
}

function Invoke-RestivraNative {
    param(
        [Parameter(Mandatory)][string]$FilePath,
        [string[]]$Arguments = @(),
        [ValidateRange(1,3600)][int]$TimeoutSeconds = 30,
        [IO.Stream]$InputStream = $null,
        [IO.Stream]$OutputStream = $null,
        [ValidateRange(1,10737418240)][long]$OutputByteLimit = 536870912,
        [string]$WorkingDirectory = '',
        [ValidateRange(64,1048576)][int]$OutputLimit = 65536
    )
    $process = [Diagnostics.Process]::new()
    $startedProcess = $false
    $timeout = [Threading.CancellationTokenSource]::new([TimeSpan]::FromSeconds($TimeoutSeconds))
    try {
        $process.StartInfo.FileName = $FilePath
        $process.StartInfo.UseShellExecute = $false
        $process.StartInfo.CreateNoWindow = $true
        if ($WorkingDirectory) { $process.StartInfo.WorkingDirectory = $WorkingDirectory }
        $process.StartInfo.RedirectStandardOutput = $true
        $process.StartInfo.RedirectStandardError = $true
        $process.StartInfo.RedirectStandardInput = ($null -ne $InputStream)
        foreach ($argument in $Arguments) { $process.StartInfo.ArgumentList.Add($argument) }
        if (-not $process.Start()) { throw 'Native command could not start.' }
        $startedProcess = $true
        $outputTask = if ($OutputStream) {
            [Restivra.NativeIO]::CopyOutputAsync($process.StandardOutput.BaseStream, $OutputStream, $OutputByteLimit, $timeout.Token)
        } else { [Restivra.NativeIO]::ReadAsync($process.StandardOutput, $OutputLimit) }
        $errorTask = $process.StandardError.BaseStream.CopyToAsync([IO.Stream]::Null)
        $inputTask = if ($InputStream) { [Restivra.NativeIO]::WriteAsync($InputStream, $process.StandardInput.BaseStream, $timeout.Token) } else { $null }
        while (-not $process.WaitForExit(100)) {
            if ($timeout.IsCancellationRequested) { throw 'Native command exceeded its time limit.' }
            foreach ($task in @($outputTask,$errorTask,$inputTask)) {
                if ($null -ne $task -and ($task.IsFaulted -or $task.IsCanceled)) { throw 'Native command or stream failed.' }
            }
        }
        foreach ($task in @($outputTask,$errorTask,$inputTask)) {
            if ($null -ne $task -and -not $task.Wait(3000)) { throw 'Native command streams did not close.' }
        }
        if ($timeout.IsCancellationRequested) { throw 'Native command exceeded its time limit.' }
        if ($OutputStream) { return @{ExitCode=$process.ExitCode;Output='';Truncated=$false} }
        $capture = $outputTask.GetAwaiter().GetResult()
        return @{ExitCode=$process.ExitCode;Output=$capture.Text;Truncated=$capture.Truncated}
    } catch {
        # Never propagate native diagnostics or source paths from process errors.
        if ($_.Exception.Message -eq 'Native command exceeded its time limit.') { throw 'Native command exceeded its time limit.' }
        throw 'Native command or stream failed.'
    } finally {
        $timeout.Cancel()
        if ($startedProcess) { try { if (-not $process.HasExited) { $process.Kill($true);$null=$process.WaitForExit(3000) } } catch {} }
        $timeout.Dispose()
        $process.Dispose()
    }
}
