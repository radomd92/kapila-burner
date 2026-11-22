#ifndef SourceDir
  #error SourceDir must point to the complete Windows runtime stage.
#endif
#ifndef OutputDir
  #error OutputDir must point to the installer output directory.
#endif

[Setup]
AppId={{EF78D7AB-1B46-4B02-9432-14B55AE9B407}
AppName=kapila-maker
AppVersion=1.0
DefaultDirName={localappdata}\Programs\kapila-maker
DefaultGroupName=kapila-maker
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir={#OutputDir}
OutputBaseFilename=kapila-maker-windows-amd64-setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
UninstallDisplayIcon={app}\kapila-maker.exe
DisableProgramGroupPage=yes

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "french"; MessagesFile: "compiler:Languages\French.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "{#SourceDir}\*"; DestDir: "{app}"; Excludes: "SOURCE-ARCHIVES,SOURCE-ARCHIVES\*,sourcepackages,sourcepackages\*,*.source,*.src.tar.*,*.pkg.tar.*"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{group}\kapila-maker"; Filename: "{app}\kapila-maker.exe"; WorkingDir: "{localappdata}"
Name: "{autodesktop}\kapila-maker"; Filename: "{app}\kapila-maker.exe"; WorkingDir: "{localappdata}"; Tasks: desktopicon
Name: "{group}\kapila-maker-web"; Filename: "{app}\kapila-maker-web.exe"; Parameters: "-listen 127.0.0.1:8787"; WorkingDir: "{localappdata}"; Comment: "Keep the console open and open http://127.0.0.1:8787 in your browser."
Name: "{group}\Uninstall kapila-maker"; Filename: "{uninstallexe}"
