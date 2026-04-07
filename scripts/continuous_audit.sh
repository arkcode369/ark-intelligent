#!/bin/bash
# Continuous Audit Daemon
# Runs audit every 15 minutes indefinitely

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
LOG_FILE="$PROJECT_DIR/.agents/audit/audit-daemon.log"
PID_FILE="$PROJECT_DIR/.agents/audit/audit.pid"

cd "$PROJECT_DIR"

echo "🤖 Starting Autonomous Audit Daemon"
echo "===================================="
echo "Project: $PROJECT_DIR"
echo "Log: $LOG_FILE"
echo "Interval: 15 minutes"
echo ""

# Save PID
echo $$ > "$PID_FILE"

# Trap for cleanup
cleanup() {
    echo "🛑 Stopping audit daemon..."
    rm -f "$PID_FILE"
    exit 0
}

trap cleanup SIGINT SIGTERM

# Main loop
CYCLE=0
while true; do
    CYCLE=$((CYCLE + 1))
    TIMESTAMP=$(date +"%Y-%m-%d %H:%M:%S")
    
    echo "" >> "$LOG_FILE"
    echo "========================================" >> "$LOG_FILE"
    echo "Audit Cycle #$CYCLE - $TIMESTAMP" >> "$LOG_FILE"
    echo "========================================" >> "$LOG_FILE"
    
    echo "🔍 Cycle #$CYCLE at $TIMESTAMP"
    
    # Run audit
    if bash "$SCRIPT_DIR/run_audit.sh" >> "$LOG_FILE" 2>&1; then
        echo "✅ Cycle #$CYCLE completed successfully"
    else
        echo "❌ Cycle #$CYCLE failed - check log for details"
        echo "📄 Log: $LOG_FILE"
    fi
    
    # Wait 15 minutes (900 seconds)
    echo "⏳ Waiting 15 minutes before next cycle..."
    sleep 900
done
