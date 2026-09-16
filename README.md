# Beamcast GX 5.9 Ultra Realism

Beamcast GX 5.9 V1.0 Beta is a medium weight ray tracing library, and Windows application. 


5.9 retains the 5.8 GPU Dataflow renderer, the 5.7 integrated GIGA benchmark, and the 5.6 multicore pipeline, while adding a deliberately high-fidelity **Ultra Realism** render preset and a cleaned-up native Win32 inspector layout.

## Ultra Realism preset

Available from:

`Render -> Quality -> Ultra Realism`

The preset is intentionally not “turn every effect to maximum.” It favors physically-oriented path tracing settings over screen-space decoration.

CPU / Auto research transport:
- 2560 x 1440 native output
- 256 base samples per pixel
- 16 path bounces
- path tracing integrator
- Halton low-discrepancy sampling
- power-heuristic MIS
- power-weighted emissive-light selection
- tight adaptive convergence threshold
- Russian roulette delayed to bounce 6
- no firefly clamp, avoiding that source of radiance bias
- no HBAO/HBAO+ overlay
- no FXAA/TXAA/TSAA blur stack
- native resolution, no reconstruction upscale
- guide-aware denoise enabled
- low bloom
- ACES-style HDR tone mapping
- SAH BVH with 32 bins
- 8 x 8 center-first tiles
- final-only progressive mode for throughput

Explicit OpenCL GPU mode keeps the same resolution, sample count, bounce depth, HDR and final-quality intent, but switches the transport controls that the portable GPU kernel does not implement to its GPU-compatible equivalents. This prevents selecting Ultra Realism from silently forcing an explicit-GPU user back onto the CPU.

## UI layout cleanup

The right inspector has been rebuilt around a shared layout geometry:
- panel width increased from 440 to 460 px
- 22 px left/right content padding
- 10 px spacing between two-column controls
- consistent 30 px button height
- consistent heading-to-control spacing
- click hitboxes are generated from the same rectangles used for painting
- Scene and Telemetry combined into a compact lower section
- duplicate status-line drawing removed
- bottom telemetry moved upward to avoid footer collisions
- longer summary strings get slightly more horizontal room

The result is less cramped and substantially less likely to exhibit text/button overlap at the default window size.

## Existing renderer systems retained

- CPU path tracing, recursive ray tracing and photon mapping
- GPU OpenCL path tracing with persistent runtime resources
- stackless threaded GPU BVH traversal
- compact GPU geometry/material records
- GPU first-hit guide generation
- multi-core CPU scheduler and parallel post stack
- MIS, power-light sampling, adaptive sampling and low-discrepancy sequences
- FXAA / TAA / TSAA / TXAA-style controls
- HBAO / HBAO+
- HDR / bloom
- DLSS-style research reconstruction
- preset editable objects and viewport dragging
- OBJ import
- PPM / BMP / PFM export
- integrated GIGA benchmark menu

## Important realism note

“Ultra Realism” is a quality preset, not a guarantee of photorealism. Scene geometry, material models, light data, textures, camera model, spectral effects and asset quality still determine how realistic an image can become. The portable renderer remains a research renderer rather than a replacement for mature production renderers.
