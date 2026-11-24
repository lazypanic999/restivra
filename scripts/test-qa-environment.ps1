#Requires -Version 7.2
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'qa-environment.ps1')
$original=Get-RestivraQAEnvironment
try{
    $env:RESTIVRA_AUTH_TOKEN='SYNTHETIC_PARENT_TOKEN'
    $env:POSTGRES_PASSWORD='SYNTHETIC_PARENT_PASSWORD'
    $env:RESTIVRA_WEBHOOK_URL='https://example.com/must-not-be-used'
    $env:COMPOSE_ENV_FILES='must-not-be-read.env'
    $snapshot=Enter-RestivraQAEnvironment 'restivra-qa-environment' 27488
    foreach($key in @('RESTIVRA_AUTH_TOKEN','POSTGRES_PASSWORD','RESTIVRA_WEBHOOK_URL','COMPOSE_ENV_FILES')){
        if($null -ne [Environment]::GetEnvironmentVariable($key,'Process')){throw 'A fixture inherited operator configuration.'}
    }
    if($env:COMPOSE_PROJECT_NAME -cne 'restivra-qa-environment' -or $env:RESTIVRA_PORT -cne '27488'){throw 'Fixture identity was not selected.'}
    $env:RESTIVRA_IMAGE='synthetic-child-selector'
    Restore-RestivraQAEnvironment $snapshot
    if($env:RESTIVRA_AUTH_TOKEN -cne 'SYNTHETIC_PARENT_TOKEN' -or $env:POSTGRES_PASSWORD -cne 'SYNTHETIC_PARENT_PASSWORD' -or $env:COMPOSE_ENV_FILES -cne 'must-not-be-read.env' -or $env:RESTIVRA_IMAGE -cne $original['RESTIVRA_IMAGE']){throw 'Operator environment was not restored exactly.'}
    Write-Host 'QA isolation passed: inherited credentials/webhooks/Compose env files cleared and restored without logging values.'
}finally{Restore-RestivraQAEnvironment $original}
