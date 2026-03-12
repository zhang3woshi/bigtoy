param(
    [double]$MinCoverage = 85
)

$ErrorActionPreference = "Stop"
$profileFile = "coverage_services"

go test ./services -coverprofile=$profileFile

$summary = go tool cover -func $profileFile | Select-String -Pattern "^total:"
if (-not $summary) {
    throw "Failed to parse Go coverage summary."
}

if ($summary.Line -notmatch "([0-9]+(?:\.[0-9]+)?)%") {
    throw "Failed to parse coverage percentage from summary: $($summary.Line)"
}

$coverage = [double]$matches[1]
Write-Host ("Backend services coverage: {0:N1}%" -f $coverage)

if ($coverage -lt $MinCoverage) {
    throw ("Coverage check failed: {0:N1}% < {1:N1}%" -f $coverage, $MinCoverage)
}
