param(
    [Parameter(Mandatory)] [string]$DisplayName,
    [Parameter(Mandatory)] [string]$Program
)

$ErrorActionPreference = "Stop"
New-Item -ItemType Directory -Force -Path (Split-Path -LiteralPath $Program) | Out-Null

try {
    $rule = Get-NetFirewallRule -DisplayName $DisplayName -ErrorAction SilentlyContinue
    if ($rule) {
        Get-NetFirewallApplicationFilter -AssociatedNetFirewallRule $rule |
            Set-NetFirewallApplicationFilter -Program $Program
        Write-Host "Updated firewall rule '$DisplayName' -> $Program"
        return
    }

    New-NetFirewallRule `
        -DisplayName $DisplayName `
        -Direction Inbound `
        -Action Allow `
        -Program $Program `
        -Profile Private, Domain `
        -Description "Local VS Code / go debug binary. Not used in Railway or Docker deployments."
    Write-Host "Created firewall rule '$DisplayName' -> $Program"
}
catch {
    Write-Warning "Could not apply firewall rule '$DisplayName' (run this VS Code task elevated once): $_"
    exit 0
}
