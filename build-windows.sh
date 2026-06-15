#!/bin/bash
set -e

echo "=== Building Rum for Windows ==="

# 1. Cross‑compile the Windows binary
wails build -platform windows/amd64 -clean

# 2. Ensure the license file exists (or skip if you don't want it)
if [ ! -f "LICENSE" ]; then
    echo "⚠️  No LICENSE file found. Creating a placeholder (MIT)."
    cat > LICENSE <<EOF
MIT License
Copyright (c) 2025 tiredbooy
Permission is hereby granted, free of charge, to any person obtaining a copy...
EOF
fi

# 3. Build the installer using Docker (Inno Setup)
docker run --rm -v "$PWD":/work amake/innosetup installer.iss

echo "✅ Windows installer created: build/bin/Rum-Setup.exe"