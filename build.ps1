$ErrorActionPreference = "Stop"
New-Item -ItemType Directory -Force -Path dist | Out-Null
Write-Host "Building WordBombGUI.exe ..."
go build -mod=vendor -ldflags "-H windowsgui -s -w" -o dist\WordBombGUI.exe .\cmd\wordbombgui
Write-Host "Building WordBombCLI.exe ..."
go build -mod=vendor -ldflags "-s -w" -o dist\WordBombCLI.exe .\cmd\wordbombcli
Write-Host "Done. Output in dist\"
