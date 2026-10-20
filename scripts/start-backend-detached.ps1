$ErrorActionPreference = "Stop"
Set-Location "D:\work_claude\WeKnora"

Get-Content ".env" | ForEach-Object {
    $line = $_.Trim()
    if ($line -eq "" -or $line.StartsWith("#")) { return }
    $idx = $line.IndexOf("=")
    if ($idx -lt 1) { return }
    $key = $line.Substring(0, $idx).Trim()
    $val = $line.Substring($idx + 1).Trim()
    Set-Item -Path "Env:$key" -Value $val
}

$env:DB_HOST = "127.0.0.1"
$env:DOCREADER_ADDR = "127.0.0.1:50051"
$env:REDIS_ADDR = "127.0.0.1:6379"
$env:NEO4J_URI = "bolt://127.0.0.1:7687"
$env:NEO4J_ENABLE = "true"
if (-not $env:CORS_ALLOWED_ORIGINS) {
    $env:CORS_ALLOWED_ORIGINS = "http://localhost:5173"
}
if (-not $env:LOCAL_STORAGE_BASE_DIR -or $env:LOCAL_STORAGE_BASE_DIR -eq "/data/files") {
    $env:LOCAL_STORAGE_BASE_DIR = "D:\work_claude\WeKnora\.local-data\files"
}
New-Item -ItemType Directory -Force -Path $env:LOCAL_STORAGE_BASE_DIR | Out-Null
New-Item -ItemType Directory -Force -Path "D:\work_claude\WeKnora\logs" | Out-Null
New-Item -ItemType Directory -Force -Path "D:\work_claude\WeKnora\tmp" | Out-Null

Get-Process WeKnora -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Seconds 1

$proc = Start-Process -FilePath "D:\work_claude\WeKnora\WeKnora.exe" `
    -WorkingDirectory "D:\work_claude\WeKnora" `
    -RedirectStandardOutput "D:\work_claude\WeKnora\logs\backend.out.log" `
    -RedirectStandardError "D:\work_claude\WeKnora\logs\backend.err.log" `
    -PassThru -WindowStyle Hidden

$proc.Id | Set-Content "D:\work_claude\WeKnora\tmp\backend.pid"
Write-Output "STARTED_PID=$($proc.Id)"
