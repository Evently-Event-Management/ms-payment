# Migration runner script for payment-gateway
# Usage: .\migrate.ps1 [environment]
# Example: .\migrate.ps1 dev

param(
    [string]$environment = "dev"
)

Write-Host "Running migration for $environment environment"

# Load environment variables from .env file
$envFile = ".env.$environment"
if (-not (Test-Path $envFile)) {
    $envFile = ".env"
    if (-not (Test-Path $envFile)) {
        Write-Host "No .env file found, using default values"
    }
    else {
        Write-Host "Loading environment from .env"
    }
}
else {
    Write-Host "Loading environment from $envFile"
}

# Load environment variables if file exists
if (Test-Path $envFile) {
    Get-Content $envFile | ForEach-Object {
        if ($_ -match '^\s*([^#].+?)\s*=\s*(.+?)\s*$') {
            $name = $matches[1]
            $value = $matches[2]
            Set-Item -Path env:$name -Value $value
        }
    }
}

# Database connection details
$DB_HOST = if ($env:DB_HOST) { $env:DB_HOST } else { "localhost" }
$DB_PORT = if ($env:DB_PORT) { $env:DB_PORT } else { "3306" }
$DB_USER = if ($env:DB_USERNAME) { $env:DB_USERNAME } else { "payment_user" }
$DB_PASS = if ($env:DB_PASSWORD) { $env:DB_PASSWORD } else { "payment_pass" }
$DB_NAME = if ($env:DB_NAME) { $env:DB_NAME } else { "payment_gateway" }

Write-Host "Connecting to MySQL at $DB_HOST`:$DB_PORT as $DB_USER"

# Check if mysql command is available
if (Get-Command mysql -ErrorAction SilentlyContinue) {
    # Run migration using mysql command
    Get-Content migrate.sql | mysql -h $DB_HOST -P $DB_PORT -u $DB_USER -p$DB_PASS $DB_NAME
}
elseif (Test-Path "C:\Program Files\MySQL\MySQL Server 8.0\bin\mysql.exe") {
    # Try a common MySQL installation path
    Get-Content migrate.sql | & "C:\Program Files\MySQL\MySQL Server 8.0\bin\mysql.exe" -h $DB_HOST -P $DB_PORT -u $DB_USER -p$DB_PASS $DB_NAME
}
else {
    Write-Host "MySQL client not found. Please install MySQL client or add it to your PATH."
    exit 1
}

# Check if migration was successful
if ($LASTEXITCODE -eq 0) {
    Write-Host "Migration completed successfully"
}
else {
    Write-Host "Migration failed"
    exit 1
}
