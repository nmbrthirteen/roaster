#!/data/data/com.termux/files/usr/bin/sh
# Supervisor for the roaster on Android.
#
# Android will kill a background process the moment it decides the device is
# idle, and a kiosk that dies at 11am on day two is worse than no kiosk. Three
# things keep it up: a wake lock, a restart loop, and a log worth reading.
#
# Install once:
#   pkg install termux-api
#   termux-setup-storage
#   mkdir -p ~/roaster && cd ~/roaster
#   # copy roaster-linux-arm64, roaster.json and events/ here
#   chmod +x roaster-linux-arm64 start.sh
#   mkdir -p ~/.termux/boot && ln -s ~/roaster/start.sh ~/.termux/boot/roaster
#
# Then install Termux:Boot from F-Droid so this runs when the tablet restarts.

cd "$(dirname "$0")" || exit 1
LOG=roaster.log

termux-wake-lock

while true; do
  echo "$(date '+%Y-%m-%d %H:%M:%S') starting" >> "$LOG"
  ./roaster-linux-arm64 >> "$LOG" 2>&1
  echo "$(date '+%Y-%m-%d %H:%M:%S') exited with $?, restarting in 3s" >> "$LOG"
  sleep 3

  # Keep the log from filling the tablet over a multi-day event.
  if [ "$(wc -c < "$LOG")" -gt 5000000 ]; then
    tail -c 1000000 "$LOG" > "$LOG.tmp" && mv "$LOG.tmp" "$LOG"
  fi
done
