$ErrorActionPreference = 'Stop'
$projectDir = Split-Path -Parent $PSScriptRoot
$base = 'http://127.0.0.1:7474'
Push-Location $projectDir
try {
    & (Join-Path $PSScriptRoot 'demo.ps1')
    Copy-Item -LiteralPath (Join-Path $projectDir 'work/demo.dump') -Destination (Join-Path $projectDir 'backups/demo.dump') -Force
    $session = Invoke-RestMethod "$base/api/session"
    $headers = @{ 'X-CSRF-Token' = $session.csrf_token }
    $existing = Invoke-RestMethod "$base/api/projects"
    foreach ($demo in @(
        @{name='Demo · Atlas'; sql='SELECT count(*) = 2 FROM public.customers'},
        @{name='Demo · Kuzey'; sql='SELECT count(*) > 100 FROM public.customers'}
    )) {
        if ($existing | Where-Object name -eq $demo.name) { continue }
        $config = $session.default_config
        $config.checks = @(@{name='Müşteri sayısı kontrolü'; sql=$demo.sql})
        $body = @{name=$demo.name; backup_pattern='demo.dump'; timezone='Europe/Istanbul'; schedule_time=''; config=$config} | ConvertTo-Json -Depth 10
        $project = Invoke-RestMethod "$base/api/projects" -Method Post -Headers $headers -ContentType 'application/json' -Body $body
        $jobBody = @{project_id=$project.id} | ConvertTo-Json
        Invoke-RestMethod "$base/api/jobs" -Method Post -Headers $headers -ContentType 'application/json' -Body $jobBody | Select-Object id,status,project_name
    }
    Write-Host 'Synthetic demo projects queued. They are labeled Demo in the panel; no schedules enabled.'
}
finally { Pop-Location }
