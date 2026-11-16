; Instalador de madPost (Inno Setup 6).
; Compilar:  iscc /DAppVersion=1.0.0 installer\madPost.iss
; Espera el binario en dist\madPost.exe (lo hace el workflow de release).

#ifndef AppVersion
  #define AppVersion "0.0.0"
#endif

[Setup]
AppId={{6B0E4C1A-9D3F-4E52-8C7A-2F1D5B6A9E40}
AppName=madPost
AppVersion={#AppVersion}
AppPublisher=madkoding
AppPublisherURL=https://github.com/madkoding/madPost
DefaultDirName={autopf}\madPost
DefaultGroupName=madPost
DisableProgramGroupPage=yes
; Por usuario: no pide administrador e instala en %LOCALAPPDATA%\Programs.
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
; Si madPost está abierto (mismo mutex que usa la app), pide cerrarlo.
AppMutex=madPost.SingleInstance
SetupIconFile=..\icon.ico
UninstallDisplayIcon={app}\madPost.exe
OutputDir=..\dist
OutputBaseFilename=madPost-setup-{#AppVersion}
Compression=lzma2
SolidCompression=yes
WizardStyle=modern

[Languages]
Name: "spanish"; MessagesFile: "compiler:Languages\Spanish.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "autostart"; Description: "Iniciar madPost con Windows"
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; Flags: unchecked

[Files]
Source: "..\dist\madPost.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\madPost"; Filename: "{app}\madPost.exe"
Name: "{autodesktop}\madPost"; Filename: "{app}\madPost.exe"; Tasks: desktopicon

[Registry]
; El mismo valor que escribe el menú "Iniciar con Windows" de la app.
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; \
  ValueName: "madPost"; ValueData: """{app}\madPost.exe"""; Tasks: autostart

[Run]
Filename: "{app}\madPost.exe"; Description: "{cm:LaunchProgram,madPost}"; Flags: nowait postinstall skipifsilent

[Code]
// Al desinstalar se quita el autoinicio aunque se haya activado desde la app.
// Las notas (%APPDATA%\madPost) se conservan.
procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usPostUninstall then
    RegDeleteValue(HKCU, 'Software\Microsoft\Windows\CurrentVersion\Run', 'madPost');
end;
