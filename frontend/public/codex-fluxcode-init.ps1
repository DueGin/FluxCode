$ErrorActionPreference = 'Stop'

$apiKey = $env:OPENAI_API_KEY
if ([string]::IsNullOrWhiteSpace($apiKey)) {
  Write-Error 'OPENAI_API_KEY 未设置或为空。请先设置环境变量 OPENAI_API_KEY。'
  exit 1
}

$baseUrl = $env:FLUXCODE_BASE_URL
if ([string]::IsNullOrWhiteSpace($baseUrl)) {
  $baseUrl = 'https://flux-code.cc'
}
$baseUrl = $baseUrl.TrimEnd('/')

$codexDir = Join-Path $env:USERPROFILE '.codex'
if (-not (Test-Path -LiteralPath $codexDir)) {
  New-Item -ItemType Directory -Path $codexDir | Out-Null
}

$authFile = Join-Path $codexDir 'auth.json'
$configFile = Join-Path $codexDir 'config.toml'

$authJson = (@{ OPENAI_API_KEY = $apiKey } | ConvertTo-Json -Compress)
[System.IO.File]::WriteAllText($authFile, $authJson, [System.Text.UTF8Encoding]::new($false))

$configToml = @"
model_provider = "fluxcode"
model = "gpt-5.2-codex"
model_reasoning_effort = "medium"

[model_providers.fluxcode]
name = "fluxcode"
base_url = "$baseUrl"
wire_api = "responses"
requires_openai_auth = true
"@

[System.IO.File]::WriteAllText($configFile, $configToml, [System.Text.UTF8Encoding]::new($false))

Write-Host '==========='
Write-Host 'Codex FluxCode 初始化完成！'
Write-Host "文件位置: $codexDir"
Write-Host '==========='

