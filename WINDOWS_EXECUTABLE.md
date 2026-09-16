# Beamcast Studio 5.9 Ultra Realism - Windows executable

GUI:
`bin/windows-x64/BeamcastStudio.exe`

CLI:
`bin/windows-x64/beamcast-cli.exe`

The GUI is a native PE32+ x86-64 Win32 application. OpenCL is loaded dynamically at runtime.

## 5.9 additions

- `Render -> Quality -> Ultra Realism`
- native 1440p / 256 spp / 16-bounce realism preset
- CPU/Auto fidelity path using MIS + power-light sampling + Halton + tight adaptive convergence
- explicit-GPU compatibility variant that remains on the OpenCL path
- physically-oriented preset choices: HBAO off, post-AA blur minimized, no firefly clamp, native resolution
- ACES-style HDR output and low bloom
- 460 px inspector panel
- consistent 22 px horizontal padding and 10 px column gap
- consolidated Scene / Telemetry section
- duplicate status draw fixed
- paint/click geometry unified for panel controls

CLI quality spelling:

`--quality ultra-realism`
