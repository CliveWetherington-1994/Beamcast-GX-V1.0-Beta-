# Beamcast GX 4.2 Quick Start

This is the short path. Use Beamcast Studio for interactive work or `beamcast::easy` in your own program; drop into HX/GX only when you need the research controls.

## 0. Launch the desktop app

Windows (MinGW-w64):

```bat
BUILD_WINDOWS.bat
RUN_STUDIO.bat
```

Linux/X11:

```bash
cmake -S . -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build -j
./build/beamcast-studio
```

The GUI and `beamcast-cli` are built by default. Disable them with `-DBEAMCAST_BUILD_STUDIO=OFF -DBEAMCAST_BUILD_CLI=OFF` for a library-only build.


## 1. Build the library

```bash
cmake -S . -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build -j
```

To build examples and tests too:

```bash
cmake -S . -B build-dev -DCMAKE_BUILD_TYPE=Release \
  -DBEAMCAST_BUILD_EXAMPLES=ON \
  -DBEAMCAST_BUILD_TESTS=ON
cmake --build build-dev -j
ctest --test-dir build-dev --output-on-failure
```

## 2. Render a scene

```cpp
#include <beamcast/beamcast.hpp>

int main() {
    using namespace beamcast;

    easy::Scene scene;
    auto white = scene.diffuse({0.75f,0.75f,0.75f});
    auto red   = scene.diffuse({0.8f,0.1f,0.08f});
    auto glass = scene.glass(1.5f);
    auto light = scene.light({14,12,10});

    scene.sphere({0,-1001,-3},1000,white)
         .sphere({-1,0,-3},1,red)
         .sphere({1,0,-3},1,glass)
         .sphere({0,5,-3},0.8f,light);

    easy::Camera camera;
    camera.position={6,3,5};
    camera.target={0,0,-3};
    camera.vertical_fov_degrees=38;

    auto options=easy::Options::preview(1280,720);
    auto result=easy::render(scene,camera,options);
    result.save_ppm("render.ppm");
}
```

The scene BVH is built automatically. If you add geometry later, it is rebuilt automatically on the next `easy::render()`.

## 3. Quality presets

- `Options::draft()` - 1 spp, very fast iteration.
- `Options::preview()` - 4 spp, denoised preview.
- `Options::balanced()` - 8 spp, general-purpose default.
- `Options::high()` - 24 spp, higher-quality output.
- `Options::ultra()` - 64 spp and deeper paths.

You can still set `thread_count`, denoising, variable-rate sampling, foveation, direct-lighting mode, and the random seed on the returned `Options` object.

## 4. Progress

```cpp
auto result=easy::render(scene,camera,options,[](float progress){
    std::cout << int(progress*100.0f) << "%\r";
});
```

Progress callbacks are serialized. If your callback throws, Beamcast returns that exception to the caller instead of terminating a render worker.

## 5. Save output

```cpp
result.save_ppm("render.ppm");       // binary P6, compact and fast
result.save_ppm("render_ascii.ppm", false); // ASCII P3
```

The generic API also exposes `beamcast::save_ppm(path, framebuffer)`.

## 6. Advanced layers

Use `beamcast::hx::PackedScene` when you want direct control over packed CPU rendering and BVH construction. Use `beamcast::gx` for wavefront rendering, ReSTIR research, reconstruction, GPU payloads, streaming, and backend experiments.
