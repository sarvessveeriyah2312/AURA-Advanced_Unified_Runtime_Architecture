#!/bin/sh
# Removes the Tread SDK installed by the macOS package.
#   sudo /Library/Tread/uninstall.sh
set -eu
if [ "$(id -u)" != 0 ]; then
	echo "Run with sudo: sudo /Library/Tread/uninstall.sh" >&2
	exit 1
fi
rm -rf /Library/Tread/bin /Library/Tread/docs /Library/Tread/release
rm -f /etc/paths.d/tread
rm -f /Library/Tread/uninstall.sh
rmdir /Library/Tread 2>/dev/null || echo "Kept /Library/Tread: it contains files not installed by Tread."
pkgutil --forget dev.tread.sdk >/dev/null 2>&1 || true
echo "Tread has been removed. Open a new terminal for the PATH change to take effect."
