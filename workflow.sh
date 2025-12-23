#!/bin/bash

set -e

echo "Running update_database.py..."
uv run python libs/update_database.py

echo "Running process.py..."
uv run python libs/process.py

echo "Running release.py..."
uv run python libs/release.py

echo "All scripts executed successfully."
