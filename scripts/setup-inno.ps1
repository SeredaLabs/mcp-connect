$ErrorActionPreference = 'Stop'
# Official compiler, pinned independently of runner-image updates.
$compilerRoot = Join-Path $env:RUNNER_TEMP 'inno-7.1.0'
$installer = Join-Path $env:RUNNER_TEMP 'innosetup-7.1.0-x64.exe'
Invoke-WebRequest 'https://github.com/jrsoftware/issrc/releases/download/is-7_1_0/innosetup-7.1.0-x64.exe' -OutFile $installer
$expected = '0362a383ed217d4c4239b5933866dd96d3eb2102737da92f80f6057a4b40df2f'
if ((Get-FileHash $installer -Algorithm SHA256).Hash.ToLowerInvariant() -ne $expected) {
    throw 'Inno Setup compiler checksum mismatch'
}
$process = Start-Process $installer -ArgumentList @('/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART', '/SP-', "/DIR=$compilerRoot") -Wait -PassThru
if ($process.ExitCode -ne 0) { throw "Compiler installation failed: $($process.ExitCode)" }
$compiler = Join-Path $compilerRoot 'ISCC.exe'
if (-not (Test-Path $compiler)) { throw 'Inno Setup compiler missing' }
"ISCC=$compiler" | Out-File $env:GITHUB_ENV -Encoding utf8 -Append
