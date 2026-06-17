$ErrorActionPreference = 'Stop'
$Repo = 'mmmnt/flmnt-cli'
$Bin = 'flmnt'

if (-not [System.Environment]::Is64BitOperatingSystem) { throw 'unsupported architecture (64-bit required)' }
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }

$version = $env:FLMNT_VERSION
if (-not $version) {
	$version = (Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest").tag_name
}
$ver = $version.TrimStart('v')
$asset = "${Bin}_${ver}_windows_${arch}.zip"
$base = "https://github.com/$Repo/releases/download/$version"

$tmp = Join-Path $env:TEMP ('flmnt-' + [guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
	Write-Host "Downloading $asset ($version)..."
	Invoke-WebRequest "$base/$asset" -OutFile (Join-Path $tmp $asset)
	Invoke-WebRequest "$base/${Bin}_${ver}_checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt')

	$line = Get-Content (Join-Path $tmp 'checksums.txt') | Where-Object { $_ -match [regex]::Escape($asset) }
	$expected = ($line -split '\s+')[0]
	if (-not $expected) { throw "no checksum found for $asset" }
	$actual = (Get-FileHash (Join-Path $tmp $asset) -Algorithm SHA256).Hash
	if ($expected.ToLower() -ne $actual.ToLower()) { throw "checksum mismatch for $asset" }

	Expand-Archive -Path (Join-Path $tmp $asset) -DestinationPath $tmp -Force
	$dir = if ($env:FLMNT_INSTALL_DIR) { $env:FLMNT_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\flmnt' }
	New-Item -ItemType Directory -Path $dir -Force | Out-Null
	Copy-Item (Join-Path $tmp "$Bin.exe") (Join-Path $dir "$Bin.exe") -Force

	Write-Host "Installed $Bin to $dir\$Bin.exe"
	$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
	if ($userPath -notlike "*$dir*") {
		[Environment]::SetEnvironmentVariable('Path', "$userPath;$dir", 'User')
		Write-Host "Added $dir to your user PATH (restart your shell to use it)."
	}
} finally {
	Remove-Item -Recurse -Force $tmp
}
