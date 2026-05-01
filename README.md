# LG WebOS TV Telegram Bot

A lightweight Go-based Telegram bot designed to remotely control LG WebOS TVs. It bypasses legacy library limitations by using a custom manifest-aware WebSocket client for full permission access on modern WebOS versions (4.0+).

## Description
This bot provides a seamless way to interact with your LG TV directly from Telegram. Unlike many existing solutions that struggle with newer WebOS security models, this project utilizes a custom implementation that handles pairing and permissions correctly, ensuring reliable control even on the latest firmware.

## Key Features
- **Smart Power Management**: Turn the TV on via Wake-on-LAN (WoL) and shut it down via SSAP.
- **Efficient State Detection**: Instantly checks TV status via port polling to avoid unnecessary waits.
- **Custom Notifications**: Send toast messages to the TV screen directly from Telegram.
- **Audio & Channel Control**: Manage volume, muting, and retrieve channel lists.
- **Centralized Configuration**: All settings managed via a single `.env` file.

## Prerequisites
- **Go**: version 1.26.2 or higher.
- **LG WebOS TV**: Ensure "LG Connect Apps" is enabled in the network settings.
- **Ngrok**: Recommended for local webhook testing.

## Configuration (.env)
Create a `.env` file in the root directory with the following variables:

```env
TELEGRAM_TOKEN=your_bot_token_from_botfather
TV_IP=192.168.1.XX
TV_MAC=AA:BB:CC:DD:EE:FF
NGROK_URL=https://your-subdomain.ngrok-free.app
```

## Commands
| Command | Description |
| :--- | :--- |
| `/tvstart` | Power on the TV via WoL and clear the home screen. |
| `/tvstop` | Shut down the TV gracefully via SSAP. |
| `/tvnotify <message>` | Send a toast notification to the TV screen. |

## Getting Started

1. **Clone the repository**:
   ```bash
   git clone https://github.com/yourusername/lg-webos-bot.git
   cd lg-webos-bot
   ```

2. **Set up configuration**:
   Copy the example configuration or create your own `.env` file as shown above.

3. **Install dependencies**:
   ```bash
   go mod download
   ```

4. **Run the bot**:
   ```bash
   go run .
   ```

Once running, the bot will listen for incoming Telegram messages and translate them into LG WebOS commands.
