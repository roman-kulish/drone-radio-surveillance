# Radio Surveillance Drone Platform

A Go-based platform for collecting and analyzing radio frequency data using Software Defined Radio (SDR) devices mounted on drones. The system captures RF measurements while recording precise location and telemetry data, enabling spatial RF analysis and signal source mapping.

## Features

- Multi-device support for concurrent data collection from multiple SDRs
- Real-time signal processing and analysis
- Integrated drone telemetry recording
- Efficient data storage using SQLite with per-flight databases
- Synchronized sampling across multiple devices
- Automatic device health monitoring and recovery
- Configurable batch processing and storage

## System Architecture

The system is split into two main components:

1. **Data Collection Tool**
   - Optimized for real-time performance
   - Runs headless on drone's Single Board Computer (SBC)
   - Handles SDR devices, data acquisition, and storage
   - Resource usage optimized for data collection

2. **Visualization Tool** (TODO)
   - Runs on ground stations
   - Implements complex analysis and visualization
   - Can load/compare data from multiple flights
   - Handles advanced signal analysis and mapping

## Getting Started

### Prerequisites

- Go 1.21 or later
- RTL-SDR and/or HackRF tools (`rtl-sdr` / `hackrf` packages, Windows binaries are included)
- SQLite3

### Configuration

Device and orchestrator configuration can be customized via YAML files (see `config/` directory).

## Contributing

Contributions are welcome! Please read our [Contributing Guidelines](CONTRIBUTING.md) first.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- RTL-SDR / HackRF project and contributors
- Go SQLite3 driver maintainers

## Known Issues

- High sample rates may require CPU governor adjustments on SBCs
- GPS accuracy affects signal source location precision
- Multiple SDR devices may cause USB bandwidth issues