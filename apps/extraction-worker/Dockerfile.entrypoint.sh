#!/bin/sh
# Entrypoint for the DocLens extraction worker.
#
# Print the version of every external tool the worker depends on so a
# crash report's first 20 lines tell you exactly what was running.
set -e

echo "==> markitdown $(markitdown --version 2>&1 | head -1 || echo 'unavailable')"
echo "==> pdftoppm   $(pdftoppm -v 2>&1 | head -1 || echo 'unavailable')"
echo "==> tesseract  $(tesseract --version 2>&1 | head -1 || echo 'unavailable')"
echo "==> ffmpeg     $(ffmpeg -version 2>&1 | head -1 || echo 'unavailable')"

exec "$@"
