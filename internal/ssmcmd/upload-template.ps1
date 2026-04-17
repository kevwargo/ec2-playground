Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem

$ErrorActionPreference = "Stop"

$zipPath = "{{ .ZipPath }}"
$presignedUrl = "{{ .URL }}"

if (Test-Path $zipPath) {
    Remove-Item $zipPath -Force
}

$zipStream = [System.IO.File]::Open($zipPath, [System.IO.FileMode]::Create)
$zip = New-Object System.IO.Compression.ZipArchive($zipStream, [System.IO.Compression.ZipArchiveMode]::Create)

foreach ($sourceDir in @(
{{range .Paths}}"{{ . }}"
{{end}}
)) {
    Get-ChildItem -Path $sourceDir -Recurse -File -ErrorAction SilentlyContinue | ForEach-Object {
        {{if .Exclude -}} if ($_.FullName -match '{{ .Exclude }}') { Write-Host "Excluding: $($_.FullName)" } else { {{- end}}
        $relativePath = (Split-Path $_.FullName -NoQualifier).TrimStart('\')

        try {
            $entry = $zip.CreateEntry($relativePath)

            $entryStream = $entry.Open()
            $fileStream  = [System.IO.File]::Open($_.FullName, 'Open', 'Read', 'ReadWrite')

            $fileStream.CopyTo($entryStream)

            $fileStream.Close()
            $entryStream.Close()
        } catch {
            Write-Warning "Skipping: $($_.FullName)"
        }
        {{if .Exclude -}} } {{- end}}
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
