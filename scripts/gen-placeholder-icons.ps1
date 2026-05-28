$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.Drawing
$dir = Join-Path $PSScriptRoot "..\apps\desktop\src-tauri\icons"
if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir | Out-Null }

# 1. Draw 256x256 PNG
$bmp = New-Object System.Drawing.Bitmap(256, 256)
$g = [System.Drawing.Graphics]::FromImage($bmp)
$g.SmoothingMode = "AntiAlias"
$g.Clear([System.Drawing.Color]::FromArgb(255, 24, 24, 27))
$brush = New-Object System.Drawing.SolidBrush([System.Drawing.Color]::FromArgb(255, 250, 204, 21))
$font = New-Object System.Drawing.Font("Segoe UI", 140, [System.Drawing.FontStyle]::Bold)
$sf = New-Object System.Drawing.StringFormat
$sf.Alignment = "Center"
$sf.LineAlignment = "Center"
$rect = New-Object System.Drawing.RectangleF(0, 0, 256, 256)
$g.DrawString("ar", $font, $brush, $rect, $sf)
$g.Dispose()
$pngPath = Join-Path $dir "icon.png"
$bmp.Save($pngPath, [System.Drawing.Imaging.ImageFormat]::Png)

# 2. Build a 32x32 PNG bitmap then wrap in ICO container
$small = New-Object System.Drawing.Bitmap($bmp, 32, 32)
$ms = New-Object System.IO.MemoryStream
$small.Save($ms, [System.Drawing.Imaging.ImageFormat]::Png)
$pngBytes = $ms.ToArray()
$ms.Dispose()
$small.Dispose()
$bmp.Dispose()

$pngLen = $pngBytes.Length
$out = New-Object System.IO.MemoryStream
$w = New-Object System.IO.BinaryWriter($out)
# ICONDIR
$w.Write([uint16]0)            # reserved
$w.Write([uint16]1)            # type=ICO
$w.Write([uint16]1)            # count
# ICONDIRENTRY
$w.Write([byte]32)             # width
$w.Write([byte]32)             # height
$w.Write([byte]0)              # colorCount
$w.Write([byte]0)              # reserved
$w.Write([uint16]1)            # planes
$w.Write([uint16]32)           # bitcount
$w.Write([uint32]$pngLen)      # bytesInRes
$w.Write([uint32]22)           # imageOffset (6 + 16)
# PNG payload
$w.Write($pngBytes)
$w.Flush()
$icoBytes = $out.ToArray()
$w.Dispose()
$out.Dispose()

$icoPath = Join-Path $dir "icon.ico"
[System.IO.File]::WriteAllBytes($icoPath, $icoBytes)

Write-Host "icon.png: $pngPath"
Write-Host "icon.ico: $icoPath ($($icoBytes.Length) bytes)"
