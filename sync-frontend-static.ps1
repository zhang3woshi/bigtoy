param(
    [switch]$InstallDependencies
)

$ErrorActionPreference = "Stop"

function Invoke-CheckedCommand {
    param(
        [Parameter(Mandatory = $true)]
        [string]$FilePath,

        [Parameter(Mandatory = $true)]
        [string[]]$Arguments
    )

    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Command failed: $FilePath $($Arguments -join ' ') (exit code: $LASTEXITCODE)"
    }
}

$projectRoot = $PSScriptRoot
$frontendDir = Join-Path $projectRoot "frontend"

if (-not (Test-Path $frontendDir)) {
    throw "Frontend directory not found: $frontendDir"
}

Push-Location $frontendDir
try {
    if ($InstallDependencies -or -not (Test-Path "node_modules")) {
        Invoke-CheckedCommand -FilePath "npm.cmd" -Arguments @("install")
    }

    Invoke-CheckedCommand -FilePath "npm.cmd" -Arguments @("run", "build:backend")
}
finally {
    Pop-Location
}

Write-Host "Frontend artifacts exported to backend/static"
