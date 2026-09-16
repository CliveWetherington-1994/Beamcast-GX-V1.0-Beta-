# Beamcast GX 5.9 Ultra Realism - Start Here

## Windows

Run:

`bin/windows-x64/BeamcastStudio.exe`

## Ultra Realism

Open:

`Render -> Quality -> Ultra Realism`

Then press `F5`.

The preset targets 2560 x 1440, 256 spp and 16 bounces, so it is intentionally expensive. For interactive camera/object movement, keep the realtime movement preview enabled and let Beamcast return to the full-quality render after movement stops.

If **OpenCL GPU** is explicitly selected, Ultra Realism uses the GPU-compatible transport variant so the render stays on the GPU. CPU/Auto uses the higher-fidelity MIS + Halton research transport.

## UI layout

The right inspector now has consistent horizontal padding, control gaps and section spacing. Scene and telemetry are combined to leave more bottom clearance, and button hitboxes share the same geometry used to draw the controls.

## Useful controls

- `F5`: render
- `Esc`: cancel
- left drag: orbit camera, or drag selected object in Object Edit mode
- right drag: pan
- mouse wheel: zoom
- `[` / `]`: rotate displayed final render
- `Ctrl+O`: import OBJ
- `Ctrl+S`: save PPM

The **Benchmark** menu still contains the GIGA benchmark suites for comparing CPU scaling and GPU/backend performance.
