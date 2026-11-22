#ifndef SourceDir
  #error SourceDir must point to the complete Windows runtime stage.
#endif
#ifndef OutputDir
  #error OutputDir must point to the installer output directory.
#endif

[Setup]
AppId={{EF78D7AB-1B46-4B02-9432-14B55AE9B407}
AppName=Kapila Burner
AppVersion=1.0
DefaultDirName={localappdata}\Programs\KapilaBurner
DefaultGroupName=Kapila Burner
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir={#OutputDir}
OutputBaseFilename=kapila-burner-windows-amd64-setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
UninstallDisplayIcon={app}\kapila-burner.exe
DisableProgramGroupPage=yes

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "french"; MessagesFile: "compiler:Languages\French.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "{#SourceDir}\*"; DestDir: "{app}"; Excludes: "SOURCE-ARCHIVES,SOURCE-ARCHIVES\*,sourcepackages,sourcepackages\*,*.source,*.src.tar.*,*.pkg.tar.*"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{group}\Kapila Burner"; Filename: "{app}\kapila-burner.exe"; WorkingDir: "{localappdata}"
Name: "{autodesktop}\Kapila Burner"; Filename: "{app}\kapila-burner.exe"; WorkingDir: "{localappdata}"; Tasks: desktopicon
Name: "{group}\Kapila Burner (Browser Server)"; Filename: "{app}\kapila-burner-web.exe"; Parameters: "-listen 127.0.0.1:8787"; WorkingDir: "{localappdata}"; Comment: "Keep the console open and open http://127.0.0.1:8787 in your browser."
Name: "{group}\Uninstall Kapila Burner"; Filename: "{uninstallexe}"
