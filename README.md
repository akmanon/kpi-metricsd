# KPI Metrics Daemon

A lightweight, high-performance log monitoring daemon that extracts Key Performance Indicators (KPIs) from log files and exposes them as Prometheus metrics. Built in Go with real-time log tailing, automatic log rotation, and optional Pushgateway integration.

## 🚀 Features

- **Real-time Log Monitoring**: Continuously tails log files and processes new entries
- **Regex-based KPI Extraction**: Define custom KPIs using regular expressions
- **Prometheus Metrics**: Exposes metrics via HTTP endpoint for Prometheus scraping
- **Pushgateway Integration**: Optional metrics pushing to Prometheus Pushgateway
- **Automatic Log Rotation**: Handles log file rotation and truncation seamlessly
- **Custom Labels**: Add custom labels to metrics for better organization
- **Graceful Shutdown**: Proper signal handling and cleanup
- **High Performance**: Efficient file I/O with buffered reading/writing

## 📋 Prerequisites

- Go 1.24.3 or higher
- Prometheus (optional, for metrics collection)
- Prometheus Pushgateway (optional, for metrics pushing)

## 🛠️ Installation

### Build from Source

```bash
# Clone the repository
git clone https://github.com/akmanon/kpi-metricsd.git
cd kpi-metricsd

# Build the application
make build

# The binary will be available at ./build/kpi-metricsd
```

### Using Makefile

```bash
# Build the application
make build

# Run with test configuration
make run

# Run tests
make test

# Clean build artifacts
make clean
```

## 📖 Usage

### Basic Usage

```bash
./build/kpi-metricsd -config="path/to/config.yaml"
```

### Configuration File

The application uses a YAML configuration file to define:
- Server settings (port, metrics path)
- Log file paths and rotation settings
- KPI definitions with regex patterns
- Pushgateway configuration (optional)

### Example Configuration

```yaml
server:
  port: 9099
  metrics_path: "/metrics"
  pushgateway:
    enabled: true
    url: "http://localhost:9091"
    job: "myjob"
    instance: "localhost"

log_config:
  source_log_file: "logs/app.log"
  redirect_log_file: "logs/app_redirect.log"
  rotated_log_file: "logs/app_rotated.log"
  rotation_interval: "1m"

kpis:
  - name: "error_count"
    regex: "^.*ERROR.*$"
    custom_labels:
      environment: "production"
      service: "webapp"
  
  - name: "warning_count"
    regex: "^.*WARN.*$"
    custom_labels:
      environment: "production"
      service: "webapp"
  
  - name: "api_requests"
    regex: "^.*API Request.*$"
    custom_labels:
      endpoint: "api"
```

## 🔧 Configuration Options

### Server Configuration

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `port` | int | HTTP server port for metrics endpoint | Required |
| `metrics_path` | string | Path for Prometheus metrics endpoint | Required |
| `pushgateway.enabled` | bool | Enable Pushgateway integration | false |
| `pushgateway.url` | string | Pushgateway URL | Required if enabled |
| `pushgateway.job` | string | Job name for Pushgateway | Required if enabled |
| `pushgateway.instance` | string | Instance name for Pushgateway | Required if enabled |

### Log Configuration

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `source_log_file` | string | Path to the source log file to monitor | Required |
| `redirect_log_file` | string | Path for redirected log content | Required |
| `rotated_log_file` | string | Path for rotated log content | Required |
| `rotation_interval` | string | Log rotation interval (e.g., "1m", "5m") | Required, min 60s |

### KPI Configuration

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `name` | string | KPI metric name | Required |
| `regex` | string | Regular expression pattern to match | Required |
| `custom_labels` | map | Custom labels for the metric | Optional |

## 📊 Metrics

The application exposes Prometheus metrics with the following format:

```
# HELP {kpi_name} count of {kpi_name} events from log monitoring
# TYPE {kpi_name} gauge
{kpi_name}{custom_labels} {count}
```

### Example Metrics Output

```
# HELP error_count count of error_count events from log monitoring
# TYPE error_count gauge
error_count{environment="production",service="webapp"} 42

# HELP api_requests count of api_requests events from log monitoring
# TYPE api_requests gauge
api_requests{endpoint="api"} 156
```

## 🔄 How It Works

1. **Log Tailing**: The application continuously monitors the source log file for new entries
2. **Log Redirection**: New log entries are redirected to a temporary file
3. **Log Rotation**: At configured intervals, the redirected log is rotated to a processing file
4. **KPI Processing**: The rotated log file is scanned for KPI patterns using regex
5. **Metrics Generation**: Matched KPIs are counted and exposed as Prometheus metrics
6. **Pushgateway**: Metrics are optionally pushed to Prometheus Pushgateway

## 🧪 Testing

Run the test suite:

```bash
make test
```

Or run specific tests:

```bash
go test -v ./internal/logmetrics
go test -v ./internal/logtail
go test -v ./internal/logrotate
```

## 📁 Project Structure

```
kpi-metricsd/
├── main.go                 # Application entry point
├── makefile               # Build and development commands
├── go.mod                 # Go module dependencies
├── README.md              # This file
├── build/                 # Build output directory
├── internal/              # Internal application code
│   ├── app/              # Main application logic
│   ├── config/           # Configuration management
│   ├── logmetrics/       # Metrics generation and Prometheus integration
│   ├── logrotate/        # Log rotation logic
│   ├── logtail/          # Log tailing and redirection
│   └── testdata/         # Test configuration and data
└── .github/              # GitHub Actions workflows
```

## 🚀 Deployment

### Docker (Example)

```dockerfile
FROM golang:1.24.3-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o kpi-metricsd .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/kpi-metricsd .
COPY config.yaml .
CMD ["./kpi-metricsd", "-config=config.yaml"]
```

### Systemd Service (Example)

```ini
[Unit]
Description=KPI Metrics Daemon
After=network.target

[Service]
Type=simple
User=kpi-metricsd
ExecStart=/usr/local/bin/kpi-metricsd -config=/etc/kpi-metricsd/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```