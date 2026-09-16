@echo off
cd /d "%~dp0"
if not exist bin\windows-x64\BeamcastStudio.exe (
  echo BeamcastStudio.exe is missing. Rebuilding it...
  call BUILD_WINDOWS.bat
  if errorlevel 1 exit /b 1
)
start "Beamcast Studio 5.3 HyperOptimized" "%~dp0bin\windows-x64\BeamcastStudio.exe"
