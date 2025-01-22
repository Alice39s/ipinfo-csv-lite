#!/bin/bash

set -e

echo "Running update_database.py..."
chmod +x update_database.py
python update_database.py

echo "Running process.py..."
chmod +x process.py
python process.py

echo "Running release.py..."
chmod +x release.py
python release.py

echo "All scripts executed successfully."
