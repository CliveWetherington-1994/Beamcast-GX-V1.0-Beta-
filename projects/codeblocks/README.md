# Code::Blocks

Open `Beamcast.workspace` in Code::Blocks on Windows with a MinGW-w64 compiler configured.

- **BeamcastStudio** builds the native Win32 GUI to `bin/windows-x64/BeamcastStudio.exe`.
- **BeamcastCLI** builds the console renderer to `bin/windows-x64/beamcast-cli.exe`.

Both targets use C++17 and compile the complete Beamcast renderer directly into the executable, avoiding separate-library linker setup.
