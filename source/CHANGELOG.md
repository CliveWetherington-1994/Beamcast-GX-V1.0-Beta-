# Changelog

## Beamcast GX 4.4

- Added the bundled advanced native Windows Studio executable distribution.
- Added progressive multithreaded Win32 portable renderer frontend with BVH traversal, OBJ import, native file dialogs, mouse orbit/pan/zoom, quality presets, cancellation, denoising, PPM/BMP export, telemetry, and final-view rotation.
- Full C++ Classic/HX/GX source remains the canonical library implementation.

## Beamcast GX 4.3

- added final-render viewport rotation in Beamcast Studio (left/right/reset)
- added View menu entries and keyboard shortcuts `[` and `]` for rotating the displayed render
- kept GUI and CLI executable targets as first-class deliverables
- refreshed versioning and Studio documentation

## 4.2.0

### Desktop application
- Added **Beamcast Studio**, a native desktop GUI target.
- Added Win32 menus, buttons, render viewport, native OBJ open/save dialogs, keyboard shortcuts, scene controls, camera controls, progress and telemetry.
- Added an X11 frontend implementing the same Studio workflow for Linux.
- Added built-in material-showcase, Cornell-like and stress-grid scenes.
- Added OBJ import with triangulation and automatic camera framing.
- Added safe asynchronous render/cancel lifecycle so scene edits cannot race active rendering.

### Tools and packaging
- Added `beamcast-cli` for headless/batch rendering.
- Added `BUILD_WINDOWS.bat` and `RUN_STUDIO.bat` for MinGW-w64 users.
- Added `docs/STUDIO.md`.
- Studio and CLI install into the normal CMake runtime destination when enabled.

### Testing
- Added `beamcast_studio_core_test`, including a real tiny render and OBJ import.
- GUI frontend launch-tested under Xvfb.
- Full suite validated with warnings-as-errors, ASan/UBSan and ThreadSanitizer.

## 4.1.0

### User experience
- Added `beamcast::easy::Scene`, `Camera`, quality presets, automatic acceleration rebuilds, normalized progress, and direct image saving.
- Added `examples/easy_scene.cpp` and `docs/QUICKSTART.md`.
- Default CMake configuration now builds only the library; examples, tests, preview, and native GPU kernels are opt-in.
- Added clear validation errors for invalid cameras, geometry, material indices, render settings, grids, and output files.

### Correctness and robustness
- Prevented negative/overflowing framebuffer allocations.
- Fixed P6 PPM parsing with CRLF headers and non-255 max values.
- Prevented divide-by-zero for zero-length sphere rays.
- Prevented null objects/materials from entering scene structures.
- Added degenerate-triangle rejection/fallback behavior.
- Invalidated packed acceleration structures when geometry changes and reject HX/GX renders until rebuilt.
- Replaced silent HX BVH traversal-stack overflow with a correctness-preserving overflow stack.
- Serialized HX progress callbacks and safely propagate callback exceptions from Classic/HX worker threads.
- Added uniform-grid dimension/overflow checks.

### Performance
- Removed two complete GX scene copies previously created solely to compute GPU payload byte counts.
- Kept framebuffer hot-path access unchecked while adding `checked_at()` for bounds-checked user access.

### Testing
- Added `beamcast_easy_test` regression coverage for the new API and bug fixes.
- Validated Release, warnings-as-errors, ASan/UBSan, and ThreadSanitizer builds.
