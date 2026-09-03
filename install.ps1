# Installs Nota from the latest GitHub release.
#
#   irm https://raw.githubusercontent.com/vishnu-kyatannawar/nota/main/install.ps1 | iex
#
# No administrator rights: the binary lands in %LOCALAPPDATA%\Programs\Nota. The
# download is checked against the release's published SHA-256 before it is used.
$ErrorActionPreference = 'Stop'

$Repo    = 'vishnu-kyatannawar/nota'
$Install = Join-Path $env:LOCALAPPDATA 'Programs\Nota'

if ([System.Environment]::Is64BitOperatingSystem -eq $false) {
    throw 'Nota requires 64-bit Windows.'
}

Write-Host 'Finding the latest release...'
$release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
$tag     = $release.tag_name
if (-not $tag) { throw "Could not determine the latest release of $Repo." }

$version = $tag.TrimStart('v')
$asset   = "nota_${version}_windows_amd64.zip"
$base    = "https://github.com/$Repo/releases/download/$tag"

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Path $tmp | Out-Null

try {
    Write-Host "Downloading Nota $tag..."
    $zip = Join-Path $tmp $asset
    Invoke-WebRequest "$base/$asset"      -OutFile $zip
    Invoke-WebRequest "$base/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt')

    # Verify before trusting the archive.
    $expected = (Get-Content (Join-Path $tmp 'checksums.txt') |
                 Where-Object { $_ -match [regex]::Escape($asset) + '$' } |
                 ForEach-Object { ($_ -split '\s+')[0] } | Select-Object -First 1)
    if (-not $expected) { throw "No checksum published for $asset." }

    $actual = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
    if ($actual -ne $expected.ToLower()) {
        throw "Checksum mismatch for $asset - refusing to install."
    }

    Expand-Archive -Path $zip -DestinationPath $tmp -Force
    New-Item -ItemType Directory -Path $Install -Force | Out-Null
    Copy-Item (Join-Path $tmp 'nota.exe') (Join-Path $Install 'nota.exe') -Force

    # Start menu shortcut.
    $startMenu = Join-Path $env:APPDATA 'Microsoft\Windows\Start Menu\Programs'
    $shortcut  = (New-Object -ComObject WScript.Shell).CreateShortcut((Join-Path $startMenu 'Nota.lnk'))
    $shortcut.TargetPath = Join-Path $Install 'nota.exe'
    $shortcut.Description = 'Daily workplans and notes, stored as plain markdown'
    $shortcut.Save()

    # Put it on PATH for future shells if it is not already there.
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ($userPath -notlike "*$Install*") {
        [Environment]::SetEnvironmentVariable('Path', "$userPath;$Install", 'User')
        Write-Host "Added $Install to your PATH - restart your shell to pick it up."
    }

    Write-Host "Installed Nota $tag to $Install\nota.exe"
    Write-Host 'Nota needs the WebView2 runtime, which ships with Windows 10 21H2 and later.'
    Write-Host 'On an older build, install it from https://developer.microsoft.com/microsoft-edge/webview2/'
}
finally {
    Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}
