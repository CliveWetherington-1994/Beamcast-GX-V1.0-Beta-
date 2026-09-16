# Beamcast with Code::Blocks + MinGW

Beamcast is C++17 and the core library has no external dependency beyond the C++ standard library and platform threading support.

## Recommended: use CMake to build, Code::Blocks to edit

From a terminal opened in the Beamcast directory, the shortest route is now:

```bat
BUILD_WINDOWS.bat
```

That produces `build-windows\BeamcastStudio.exe` and `build-windows\beamcast-cli.exe`. Run `RUN_STUDIO.bat` to launch the GUI.

For a development build with tests:

```bat
cmake -S . -B build -G "MinGW Makefiles" -DCMAKE_BUILD_TYPE=Release -DBEAMCAST_BUILD_TESTS=ON
cmake --build build -j
ctest --test-dir build --output-on-failure
```

Then open the source tree in Code::Blocks for editing. This keeps the build definition in one portable `CMakeLists.txt` instead of duplicating compiler settings in an IDE project file.

## Manual Code::Blocks project

For a static-library target:

1. Create a new **Static library** C++ project named `beamcast`.
2. Add every `.cpp` file from `src/`.
3. Add `include/` under **Project -> Build options -> Search directories -> Compiler**.
4. Set the compiler language standard to `-std=c++17`.
5. With 32-bit MinGW, enable SSE2 if the toolchain does not already do so. On 64-bit x86 builds SSE2 is normally available. Beamcast falls back to scalar vector math if SSE2 is unavailable.
6. Build the library.

For an application using Beamcast, add `include/` to the compiler search path, add the directory containing the built Beamcast library to the linker search path, and link `beamcast`.

The optional `examples/preview_glfw.cpp` additionally requires GLFW and OpenGL. Keep it out of the project unless those dependencies are installed and configured.

## Beamcast Studio in a manual Code::Blocks project

If you want the GUI as a native Code::Blocks application instead of letting CMake create it, add `app/studio_core.cpp` and `app/studio_win32.cpp` to a Windows GUI executable project, add all core `src/*.cpp` files or link the previously built `beamcast` static library, and link the Windows system libraries `user32`, `gdi32`, and `comdlg32`. The CMake build is still recommended because it keeps the library, GUI, CLI and test configuration synchronized.
