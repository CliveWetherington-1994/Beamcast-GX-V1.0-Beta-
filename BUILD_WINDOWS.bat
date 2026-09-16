@echo off
setlocal
cd /d "%~dp0"
if not exist bin\windows-x64 mkdir bin\windows-x64

where go >nul 2>nul
if errorlevel 1 (
  echo Go was not found in PATH.
  echo The prebuilt executables are already included under bin\windows-x64\.
  echo To rebuild the advanced portable Win32 executable, install Go 1.22+.
  echo To build the canonical C++ Studio instead, run BUILD_WINDOWS_CPP.bat.
  exit /b 1
)

pushd windows_portable
echo Building BeamcastStudio.exe...
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -trimpath -ldflags "-H=windowsgui -s -w" -o ..\bin\windows-x64\BeamcastStudio.exe .
if errorlevel 1 goto :fail

echo Building beamcast-cli.exe...
go build -tags beamcast_cli -trimpath -ldflags "-s -w" -o ..\bin\windows-x64\beamcast-cli.exe .
if errorlevel 1 goto :fail
popd

echo.
echo Beamcast Windows binaries rebuilt successfully.
echo GUI: bin\windows-x64\BeamcastStudio.exe
echo CLI: bin\windows-x64\beamcast-cli.exe
exit /b 0

:fail
popd
echo Windows portable build failed.
exit /b 1
