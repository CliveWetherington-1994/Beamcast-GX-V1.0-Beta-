@echo off
setlocal
cd /d "%~dp0"
where cmake >nul 2>nul || (
  echo CMake was not found in PATH.
  echo Install CMake plus MinGW-w64 or Visual Studio C++.
  exit /b 1
)
if not exist build-windows mkdir build-windows
if not exist bin\windows-x64 mkdir bin\windows-x64
cmake -S source -B build-windows -G "MinGW Makefiles" -DCMAKE_BUILD_TYPE=Release -DBEAMCAST_BUILD_STUDIO=ON -DBEAMCAST_BUILD_CLI=ON
if errorlevel 1 exit /b 1
cmake --build build-windows -j
if errorlevel 1 exit /b 1
copy /Y build-windows\BeamcastStudio.exe bin\windows-x64\BeamcastStudio.exe >nul
copy /Y build-windows\beamcast-cli.exe bin\windows-x64\beamcast-cli.exe >nul
echo.
echo Beamcast Windows build complete.
echo GUI: bin\windows-x64\BeamcastStudio.exe
echo CLI: bin\windows-x64\beamcast-cli.exe
endlocal
