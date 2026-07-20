@echo off
setlocal
if not exist dist mkdir dist
echo Building WordBombGUI.exe ...
go build -mod=vendor -ldflags "-H windowsgui -s -w" -o dist\WordBombGUI.exe .\cmd\wordbombgui
if errorlevel 1 exit /b 1
echo Building WordBombCLI.exe ...
go build -mod=vendor -ldflags "-s -w" -o dist\WordBombCLI.exe .\cmd\wordbombcli
if errorlevel 1 exit /b 1
echo Done. Output in dist\
