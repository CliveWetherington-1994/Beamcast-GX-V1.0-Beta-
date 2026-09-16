# Beamcast Studio 5.6 Multicore HyperScale portable Win32 path

The portable renderer combines the CPU research renderer, persistent OpenCL GPU backend, Win32 Studio UI, and CLI.

5.6 specifically replaces shared hot-loop ray telemetry with worker-local counters, uses dynamic one-tile claims for load balance, reduces atomic polling, batches progressive publication, and parallelizes the final tone map plus the expensive denoise/AO/bloom/AA/reconstruction passes.
