#ifndef AppVersion
  #define AppVersion "2.0.0.0"
#endif

#ifndef SourceBinDir
  #define SourceBinDir "..\\build\\bin"
#endif

#ifndef OutputDir
  #define OutputDir "..\\build\\installer"
#endif

[Setup]
AppId={{8EDE9ED8-3514-492D-AF64-4E5FC856D636}
AppName=Lightroom Sync
AppVersion={#AppVersion}
AppVerName=Lightroom Sync {#AppVersion}
DefaultDirName={localappdata}\Programs\Lightroom Sync
DefaultGroupName=Lightroom Sync
DisableProgramGroupPage=yes
UninstallDisplayIcon={app}\LightroomSyncUI.exe
OutputDir={#OutputDir}
OutputBaseFilename=LightroomSyncSetup-v{#AppVersion}-windows-amd64
Compression=lzma2
SolidCompression=yes
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
PrivilegesRequired=lowest
WizardStyle=modern
UsePreviousAppDir=yes
UsePreviousTasks=yes
CloseApplications=yes
RestartApplications=no
AppMutex=LightroomSyncAgent_Mutex

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "startupagent"; Description: "Start Lightroom Sync Agent with Windows"; GroupDescription: "Startup"; Flags: checkedonce
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional shortcuts"

[Files]
Source: "{#SourceBinDir}\LightroomSyncAgent.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceBinDir}\LightroomSyncUI.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceBinDir}\build-metadata.json"; DestDir: "{app}"; Flags: ignoreversion skipifsourcedoesntexist

[Icons]
Name: "{autoprograms}\Lightroom Sync"; Filename: "{app}\LightroomSyncUI.exe"; WorkingDir: "{app}"
Name: "{autodesktop}\Lightroom Sync"; Filename: "{app}\LightroomSyncUI.exe"; WorkingDir: "{app}"; Tasks: desktopicon

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "LightroomSync"; ValueData: """{app}\LightroomSyncAgent.exe"" --minimized"; Tasks: startupagent; Flags: uninsdeletevalue

[Run]
Filename: "{app}\LightroomSyncAgent.exe"; Parameters: "--minimized"; Description: "Start Lightroom Sync Agent"; Flags: nowait postinstall skipifsilent runasoriginaluser
Filename: "{app}\LightroomSyncUI.exe"; Description: "Open Lightroom Sync"; Flags: nowait postinstall skipifsilent unchecked runasoriginaluser

[Code]
function TryStopProcess(const ImageName: string; const ForceKill: Boolean): Boolean;
var
  ResultCode: Integer;
  Params: string;
begin
  if ForceKill then
    Params := '/IM "' + ImageName + '" /T /F'
  else
    Params := '/IM "' + ImageName + '" /T';

  Result := Exec(ExpandConstant('{sys}\taskkill.exe'), Params, '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  if Result then
    Log(Format('taskkill executed for %s (force=%d), exit=%d', [ImageName, Ord(ForceKill), ResultCode]))
  else
    Log(Format('taskkill failed to start for %s (force=%d)', [ImageName, Ord(ForceKill)]));
end;

procedure StopRunningProcesses();
begin
  Log('Stopping Lightroom Sync processes before install/uninstall...');
  TryStopProcess('LightroomSyncUI.exe', False);
  TryStopProcess('LightroomSyncAgent.exe', False);
  Sleep(1200);
  TryStopProcess('LightroomSyncUI.exe', True);
  TryStopProcess('LightroomSyncAgent.exe', True);
  Sleep(400);
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
begin
  StopRunningProcesses();
  Result := '';
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
begin
  if CurUninstallStep = usUninstall then
    StopRunningProcesses();
end;
