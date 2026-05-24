#!/bin/bash
set -e

APP_NAME="Rum"
DESKTOP_FILE="${APP_NAME}.desktop"
INSTALL_DIR="$HOME/.local/share/applications"

echo "=== Building ${APP_NAME} for Linux ==="

# 1. Build the binary
wails build -clean

# 2. Create .desktop file with the modern description
cat > "$DESKTOP_FILE" <<EOF
[Desktop Entry]
Type=Application
Name=${APP_NAME}
Comment=Powerful, fast and modern download manager
Exec=$(pwd)/build/bin/${APP_NAME}
Icon=$(pwd)/build/icon.png
Terminal=false
Categories=Network;FileTransfer;
StartupWMClass=${APP_NAME}
EOF

chmod +x "$DESKTOP_FILE"

echo "✅ Build complete!"
echo "📦 Binary: build/bin/${APP_NAME}"
echo "📄 Desktop file: ${DESKTOP_FILE}"
echo ""
echo "To add to your app menu, run:"
echo "  cp ${DESKTOP_FILE} ${INSTALL_DIR}/"
echo "  update-desktop-database ${INSTALL_DIR}"