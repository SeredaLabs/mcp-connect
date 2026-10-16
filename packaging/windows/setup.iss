#ifndef AppVersion
  #error AppVersion is required
#endif
#ifndef TargetArch
  #error TargetArch is required
#endif
#ifndef SourceDir
  #error SourceDir is required
#endif
#ifndef OutputPath
  #error OutputPath is required
#endif

[Setup]
AppId={{5B8DF8D8-55E9-4289-BA09-BD2340A328F1}
AppName=MCP Connect
AppVersion={#AppVersion}
AppPublisher=SeredaLabs
AppPublisherURL=https://github.com/SeredaLabs
AppSupportURL=https://github.com/SeredaLabs/mcp-connect
DefaultDirName={localappdata}\Programs\MCP Connect
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
#if TargetArch == "arm64"
ArchitecturesAllowed=arm64
#else
ArchitecturesAllowed=x64compatible
#endif
OutputDir={#OutputPath}
OutputBaseFilename=mcp-connect-windows-{#TargetArch}-setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
SetupIconFile=..\..\assets\icon.ico
UninstallDisplayIcon={app}\mcp-connect.exe
CloseApplications=yes
RestartApplications=no
SetupLogging=yes
LicenseFile={#SourceDir}\LICENSE.txt

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Shortcuts:"; Flags: unchecked

[Files]
Source: "{#SourceDir}\mcp-connect.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\README.uk.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\LICENSE.txt"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\THIRD_PARTY_NOTICES.txt"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{userprograms}\MCP Connect"; Filename: "{app}\mcp-connect.exe"; WorkingDir: "{app}"
Name: "{userdesktop}\MCP Connect"; Filename: "{app}\mcp-connect.exe"; WorkingDir: "{app}"; Tasks: desktopicon

[Run]
Filename: "{app}\mcp-connect.exe"; Description: "Open MCP Connect"; Flags: nowait postinstall skipifsilent
