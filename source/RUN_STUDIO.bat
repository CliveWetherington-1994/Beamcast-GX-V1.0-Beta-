@echo off
if not exist build-windows\BeamcastStudio.exe call BUILD_WINDOWS.bat
if errorlevel 1 exit /b 1
start "Beamcast Studio" build-windows\BeamcastStudio.exe
