import logging
import json
import random
import time
import os
from flask import Flask, jsonify

app = Flask(__name__)

# Configure JSON logging to stdout
class JsonFormatter(logging.Formatter):
    def format(self, record):
        # We use ISO8601 format for the timestamp to make it easier for MongoDB
        from datetime import datetime, timezone
        now = datetime.now(timezone.utc).isoformat()
        
        log_entry = {
            "timestamp": now,
            "service_name": "flask-target-app",
            "severity": record.levelname,
            "message": record.getMessage(),
            "container_id": os.environ.get('HOSTNAME', 'unknown')
        }
        return json.dumps(log_entry)

logger = logging.getLogger('target-app')
logger.setLevel(logging.INFO)
logHandler = logging.StreamHandler()
formatter = JsonFormatter()
logHandler.setFormatter(formatter)
logger.addHandler(logHandler)

# We remove the default flask logger to prevent duplicate logs and keep logs clean
import logging as builtin_logging
werkzeug_logger = builtin_logging.getLogger('werkzeug')
werkzeug_logger.setLevel(builtin_logging.ERROR) # Only log werkzeug errors

@app.route('/')
def index():
    logger.info("Handling request to /")
    return jsonify({"status": "ok"}), 200

@app.route('/error')
def trigger_error():
    error_msgs = [
        "Database connection failed",
        "Division by zero",
        "Timeout while calling external API",
        "Null pointer exception"
    ]
    msg = random.choice(error_msgs)
    logger.error(msg)
    return jsonify({"error": msg}), 500

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)
