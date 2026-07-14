# Intelligent Log & Alert Aggregator

A production-ready log and alert aggregation system built with Go, Python, Docker, and MongoDB.

## Architecture

This system consists of three main components:
1. **Target App (Python/Flask)**: A mock application generating JSON-formatted log entries.
2. **Observer Service (Go)**: A sidecar-like service that connects to the Docker socket, tails the target app's logs in real-time, filters for `ERROR` level logs, and buffers them.
3. **MongoDB**: The log sink where all log entries are batch-inserted for historical analysis.

## Root Cause Analysis (RCA)

When an incident occurs in production, finding the root cause quickly is critical. This setup aids in RCA by:
- **Centralizing Logs**: Developers don't need to SSH into multiple containers. All logs from `target-app` are automatically pushed to MongoDB.
- **Immediate Alerting**: The Go observer filters for `ERROR` logs and triggers an immediate webhook alert (e.g., to PagerDuty or Slack) with the error context.
- **Rich Context**: The JSON log format includes the `container_id`, `service_name`, and exact `timestamp`, allowing responders to pinpoint exactly where and when the error occurred.

## Historical Trend Monitoring

By persisting logs to MongoDB, Application Support Engineers can perform historical analysis:
- **Optimized Queries**: The database schema includes compound indexes on `{service_name, timestamp}` and `{severity, timestamp}`. This allows rapid querying of logs over specific time ranges.
- **Trend Spotting**: You can aggregate logs by `severity` over weeks or months to see if error rates are increasing after a new deployment.
- **Capacity Planning**: Monitoring the volume of logs helps in estimating traffic and identifying abnormal spikes in application activity.

## Getting Started

### Prerequisites
- Docker and Docker Compose

### Running Locally
1. Clone this repository.
2. Run `docker-compose up --build -d` to start the services in the background.
3. The Target App will be available at `http://localhost:5000`.

### Testing the Setup
1. Generate normal logs by visiting:
   ```bash
   curl http://localhost:5000/
   ```
2. Trigger an error and alert by visiting:
   ```bash
   curl http://localhost:5000/error
   ```
3. Check the Observer logs to see the alert firing:
   ```bash
   docker logs observer
   ```
4. Query MongoDB to verify log insertion:
   ```bash
   docker exec -it mongodb mongosh
   > use logsdb
   > db.logs.find({severity: "ERROR"}).pretty()
   ```

## CI/CD Automation

This repository includes a GitHub Actions workflow (`.github/workflows/main.yml`) that automatically:
- Builds the Docker images.
- Starts the entire stack using `docker-compose`.
- Simulates traffic to generate logs.
- Runs an integration test against the MongoDB container to ensure the Observer successfully batch-inserted the errors.
