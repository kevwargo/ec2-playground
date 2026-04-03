Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem

$ErrorActionPreference = "Stop"

# Paths
$sourceDir = "{{ .Dir }}"
$zipPath   = "{{ .ZipPath }}"

# Pre-signed S3 URL
$presignedUrl = "{{ .URL }}"

# Remove existing ZIP if it exists
if (Test-Path $zipPath) {
    Remove-Item $zipPath -Force
}

$zipStream = [System.IO.File]::Open($zipPath, [System.IO.FileMode]::Create)
$zip = New-Object System.IO.Compression.ZipArchive($zipStream, [System.IO.Compression.ZipArchiveMode]::Create)

Get-ChildItem -Path $sourceDir -Recurse -File -ErrorAction SilentlyContinue | ForEach-Object {
    if (! $_.FullName.EndsWith('.exe')) {
        $relativePath = $_.FullName.Substring($sourceDir.Length).TrimStart('\')

        try {
            $entry = $zip.CreateEntry($relativePath)

            $entryStream = $entry.Open()
            $fileStream  = [System.IO.File]::Open($_.FullName, 'Open', 'Read', 'ReadWrite')

            $fileStream.CopyTo($entryStream)

            $fileStream.Close()
            $entryStream.Close()
        }
        catch {
            Write-Warning "Skipping: $($_.FullName)"
        }
    } else {
        Write-Host "Skipping: $($_.FullName)"
    }
}

$zip.Dispose()
$zipStream.Close()

# Upload to S3 using PUT
try {
    Invoke-RestMethod -Uri $presignedUrl -Method Put -InFile $zipPath -ContentType "application/zip"
    Write-Host "Upload successful!"
}
catch {
    Write-Error "Upload failed: $_"
}

Remove-Item $zipPath -Force
