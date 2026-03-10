# FF Economic Calendar Telegram Bot

Fast, zero-dependency Go bot that sends Forex Factory economic calendar data to Telegram.
No AI, no bullshit — just raw data from ForexFactory.

## Features

- `/calendar` — Today's economic events (WIB timezone)
- `/week` — Full week calendar
- `/high` — High impact events only
- `/next` — Next 10 upcoming events with countdown
- `/refresh` — Force data refresh
- `/chatid` — Show current chat ID
- **Auto pre-alerts**: 30min, 15min, 5min before events
- **Auto result alerts**: Sends actual vs forecast when data released
- **Beat/Miss detection**: Tells you if actual > forecast or < forecast

## Data Source

Forex Factory official JSON export: `https://nfs.faireconomy.media/ff_calendar_thisweek.json`
- Refreshed every 5 minutes automatically
- Contains: title, country, impact, forecast, previous, actual, datetime
- No scraping, no Cloudflare issues

## Quick Start

### Option 1: Run directly (requires Go 1.22+)

```bash
# Clone/copy files
mkdir ff-calendar-bot && cd ff-calendar-bot

# Set environment
export BOT_TOKEN="your-telegram-bot-token"
export CHAT_ID="your-chat-id"  # get this via /chatid command

# Build and run
go build -o ffbot main.go
./ffbot
```

### Option 2: Docker

```bash
# Create .env file
cp .env.example .env
# Edit .env with your BOT_TOKEN and CHAT_ID

# Run
docker compose up -d

# Check logs
docker compose logs -f
```

### Option 3: Docker manual build

```bash
docker build -t ffbot .
docker run -d --name ffbot \
  -e BOT_TOKEN="your-token" \
  -e CHAT_ID="your-chat-id" \
  --restart always \
  ffbot
```

## Getting Your CHAT_ID

1. Start the bot without CHAT_ID (alerts won't fire, but commands work)
2. Send `/chatid` to the bot in your target chat/group
3. Copy the ID and set it as CHAT_ID env var
4. Restart the bot

For groups: add the bot to the group, send `/chatid` there.

## Deploy to VPS (cheapest method)

```bash
# On any Linux VPS ($3-5/month)
scp ffbot user@your-vps:/opt/ffbot/

# Create systemd service
sudo tee /etc/systemd/system/ffbot.service << 'EOF'
[Unit]
Description=FF Calendar Telegram Bot
After=network.target

[Service]
Type=simple
ExecStart=/opt/ffbot/ffbot
Environment=BOT_TOKEN=your-token
Environment=CHAT_ID=your-chat-id
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl enable ffbot
sudo systemctl start ffbot
sudo systemctl status ffbot
```

## Alert Behavior

### Pre-Event Alerts
Sent at 30, 15, and 5 minutes before each event:
```
== NEWS ALERT ==
[HIGH] USD in 5 minutes

!!!  USD  Non-Farm Employment Change
Time: 19:30 WIB
Forecast: 185K
Previous: 143K
```

### Result Alerts
Sent when actual data is released:
```
== NEWS RESULT ==
[HIGH] USD

USD  Non-Farm Employment Change
Time: 19:30 WIB

Actual:   256K
Forecast: 185K
Previous: 143K

BETTER than forecast
```

## Architecture

- **Zero external dependencies** — only Go stdlib
- **3 goroutines**: command polling, data refresh (5min), alert checker (30s)
- **Single binary** ~6MB compiled
- **Memory**: ~10-15MB runtime
- **Docker image**: ~15MB total (alpine + binary)

## Configuration

| Env Var | Required | Description |
|---------|----------|-------------|
| BOT_TOKEN | Yes | Telegram bot token from @BotFather |
| CHAT_ID | No* | Target chat for auto-alerts |

*Without CHAT_ID, commands still work but auto-alerts are disabled.

To modify alert timing, edit `ALERT_BEFORE` in main.go:
```go
ALERT_BEFORE = []int{30, 15, 5}  // minutes before event
```

## Cross-compile

```bash
# Linux (VPS)
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ffbot main.go

# Linux ARM (Raspberry Pi)
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ffbot main.go

# Windows
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ffbot.exe main.go
```
