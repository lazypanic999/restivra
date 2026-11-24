# Test fixtures must never inherit an operator's real webhook or credentials.
function Get-RestivraQAEnvironment {
    $snapshot=[Collections.Generic.Dictionary[string,string]]::new([StringComparer]::Ordinal)
    foreach($entry in [Environment]::GetEnvironmentVariables('Process').GetEnumerator()){
        if($entry.Key -match '^(COMPOSE_|RESTIVRA_)' -or $entry.Key -eq 'POSTGRES_PASSWORD'){$snapshot.Add([string]$entry.Key,[string]$entry.Value)}
    }
    return ,$snapshot
}
function Enter-RestivraQAEnvironment([string]$Project,[int]$Port){
    $snapshot=Get-RestivraQAEnvironment
    foreach($key in $snapshot.Keys){Remove-Item -LiteralPath "Env:$key" -ErrorAction Stop}
    $env:COMPOSE_PROJECT_NAME=$Project;$env:RESTIVRA_PORT="$Port"
    return ,$snapshot
}
function Restore-RestivraQAEnvironment($Snapshot){
    $current=Get-RestivraQAEnvironment
    foreach($key in $current.Keys){Remove-Item -LiteralPath "Env:$key" -ErrorAction Stop}
    foreach($entry in $Snapshot.GetEnumerator()){[Environment]::SetEnvironmentVariable($entry.Key,$entry.Value,'Process')}
}
