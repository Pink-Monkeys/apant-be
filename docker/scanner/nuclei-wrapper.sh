#!/bin/sh
HOME=/home/scanner
export HOME
mkdir -p /home/scanner/.config/nuclei
python3 -c "import json, os; cfg='/home/scanner/.config/nuclei/.templates-config.json'; json.dump({'nuclei-templates-directory':'/home/scanner/.nuclei-templates'}, open(cfg, 'w'))" 2>/dev/null || true
exec /usr/local/bin/nuclei-real "$@"
