# Beamcast Studio 5.3

Beamcast Studio is the native desktop application shipped with Beamcast GX. It uses the same GX/HX renderer as the library; the GUI is not a separate toy renderer.

## Windows

The Windows frontend uses the Win32 API only, so it does not require Qt, SDL, GLFW, Dear ImGui, or another GUI framework.

From a MinGW-w64 command prompt:

```bat
BUILD_WINDOWS.bat
RUN_STUDIO.bat
```

The resulting executables are:

- `build-windows\BeamcastStudio.exe` - desktop GUI
- `build-windows\beamcast-cli.exe` - batch/headless renderer

Visual Studio users can use the normal CMake workflow instead of the batch file.

## Linux / X11

```bash
cmake -S . -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build -j
./build/beamcast-studio
```

The X11 development headers are required to build the Linux GUI. The library and CLI still build if Studio is disabled with `-DBEAMCAST_BUILD_STUDIO=OFF`.

## Main UI

The viewport occupies the main area of the window. The inspector contains render settings, resolution, denoising, variable-rate sampling, camera orbit/dolly controls, final-render viewport rotation, scene statistics, progress, and telemetry.

Menus provide:

- **File**: new showcase, open OBJ, save rendered PPM, exit
- **Scene**: material showcase, Cornell-like scene, stress grid, clear
- **Render**: start, quality cycle, cancel
- **Camera**: reset, orbit, dolly
- **Help**: application information

Keyboard shortcuts:

- `F5` render
- `Esc` cancel
- `Ctrl+O` open OBJ
- `Ctrl+S` save render
- `Ctrl+N` reset to showcase (Windows)

OBJ imports are triangulated and the camera is automatically framed around the imported model.

## Headless CLI

```bash
beamcast-cli --scene showcase --quality high --width 1920 --height 1080 --output render.ppm
beamcast-cli --scene model.obj --quality preview --output model.ppm
```

Run `beamcast-cli --help` for all options.


## Final-render rotation

Studio 5.3 can rotate the displayed final render in 90-degree increments without modifying the saved image data.
Use the **View** menu, the **Rotate left / Rotate right / Reset view rotation** buttons, or the `[` and `]` keys.
This is useful when inspecting tall renders, portrait layouts, or imported scenes that are easier to review from a rotated orientation.
