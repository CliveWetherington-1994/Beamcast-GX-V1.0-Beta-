@echo off
setlocal
where cmake >nul 2>nul || (
  echo CMake was not found in PATH.
  echo Install CMake and a C++17 compiler such as MinGW-w64 or Visual Studio.
  exit /b 1
)

if not exist build-windows mkdir build-windows
cmake -S . -B build-windows -G "MinGW Makefiles" -DCMAKE_BUILD_TYPE=Release -DBEAMCAST_BUILD_STUDIO=ON -DBEAMCAST_BUILD_CLI=ON
if errorlevel 1 exit /b 1
cmake --build build-windows -j
if errorlevel 1 exit /b 1

echo.
echo Build complete.
echo GUI: build-windows\BeamcastStudio.exe
echo CLI: build-windows\beamcast-cli.exe
endlocal
