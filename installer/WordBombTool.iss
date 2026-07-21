; Inno Setup script for Word Bomb Tool (Go edition).
; Bundles the two statically-linked Go binaries -- no runtime prerequisite,
; unlike the C#/WPF port. Build the exes first:
;   build.bat
; then compile the installer:
;   "C:\Program Files (x86)\Inno Setup 6\ISCC.exe" installer\WordBombTool.iss
; Output lands in dist\installer\WordBombTool-Setup.exe.

#define MyAppName "Word Bomb Tool"
#define MyAppVersion "1.0.0"
#define MyAppPublisher "Word Bomb Tool"
#define MyAppExeName "WordBombGUI.exe"
#define MyCliExeName "WordBombCLI.exe"
; Path to this script is installer\WordBombTool.iss, so ..\ is the repo root.
#define RepoRoot "..\"
#define BinSrc RepoRoot + "dist"

[Setup]
AppId={{9B3E7B1E-2C9A-4B3D-9F6E-2B7A5C4D1E2F}}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
; Per-machine by default, but allow a per-user install too (no admin needed)
; via the privileges dialog below.
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
OutputDir=..\dist\installer
OutputBaseFilename=WordBombTool-Setup
; No standalone .ico ships in this repo (the Go build embeds its icon via
; rsrc_windows_amd64.syso directly into the exe) -- SetupIconFile needs a
; real .ico file, so the wizard just uses Inno Setup's default icon.
; UninstallDisplayIcon below still pulls the icon straight from the exe.
Compression=lzma2/max
SolidCompression=yes
WizardStyle=modern
UninstallDisplayIcon={app}\{#MyAppExeName}
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked
Name: "addtopath"; Description: "Add the CLI (WordBombCLI.exe) to PATH"; GroupDescription: "Command-line tool"; Flags: unchecked

[Files]
Source: "{#BinSrc}\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#BinSrc}\{#MyCliExeName}"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Registry]
; User-scope PATH entry (no admin required) for the CLI, only if requested.
Root: HKCU; Subkey: "Environment"; ValueType: expandsz; ValueName: "Path"; \
    ValueData: "{olddata};{app}"; Tasks: addtopath; Check: NeedsAddPath('{app}')

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent

[Code]
function NeedsAddPath(Param: string): boolean;
var
  OrigPath: string;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', OrigPath) then
  begin
    Result := True;
    exit;
  end;
  Result := Pos(';' + Param + ';', ';' + OrigPath + ';') = 0;
end;
