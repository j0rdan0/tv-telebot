# Changelog

## [1.4.2] - 2026-05-02

### Added
- Makefile: Added a Makefile to the root directory for standardizing build and run tasks.

## [1.4.1] - 2026-05-02

### Added
- **Set Channel Button**: Added a Set Channel button to the interactive control menu.
- **UI Cleanup**: Removed all emoticons from user-facing messages and bot menus.

### Fixed
- **Missing Menu Actions**: Integrated the missing channel control actions into the interactive button handler.

## [1.4.0] - 2026-05-02

### Added
- **Channel Navigation**: Added commands and interactive buttons to change channels and navigate back.
    - /tvchannel <number>: Switch to a specific channel.
    - /tvback: Toggle back to the previous channel.
    - Added Back button to the interactive control menu.
- **TV Client Enhancements**: Added SetChannel, GetCurrentChannel, and Close methods to the WebOSTV struct.

### Improved
- **State Management**: The bot now tracks the last active channel per chat, enabling the Back functionality.

## [1.3.0] - 2026-05-02

### Changed
- **Project Structure Refactor**: Adopted the standard Go project layout.
    - Moved main entry point to cmd/tv-bot/.
    - Organized core logic into internal/ packages (bot, config, ngrok, tv).
    - Improved code encapsulation and maintainability.

## [1.2.4] - 2026-05-02

### Fixed
- **Ngrok Service Authentication**: Added support for NGROK_AUTHTOKEN in the .env file. The bot now explicitly passes the authtoken to the ngrok process, ensuring successful tunnel creation when running as a systemd service or in environments without pre-configured ngrok config files.

## [1.2.3] - 2026-05-02

### Improved
- **Ngrok Debugging**: Added stderr piping for the ngrok process. Errors during automated ngrok startup (such as missing authtokens or path issues) will now appear directly in the bot's logs, making it easier to troubleshoot systemd deployments.

## [1.2.2] - 2026-05-02

### Fixed
- **Interactive Buttons**: Resolved an issue where inline buttons in the control menu were not triggering TV actions. Buttons are now fully functional.

### Changed
- **Code Refactoring**: Refactored bot command logic into reusable helper functions (handleTVStart, handleTVStop, etc.) to share logic between slash commands and interactive buttons.

## [1.2.1] - 2026-05-02

### Added
- **Audio Control Handlers**: Added /tvmute and /tvvolume commands to the Telegram bot.
    - /tvmute [on|off]: Toggles or explicitly sets the mute state.
    - /tvvolume <0-100>: Sets the TV volume level precisely.

## [1.2.0] - 2026-05-02

### Added
- **Ngrok Automation**: Created ngrok.go to automatically start an ngrok tunnel on port 8080 if one isn't already running.
- **Dynamic Webhook Configuration**: The bot now retrieves the public ngrok URL at startup and automatically configures its Telegram webhook, eliminating manual configuration steps.
- **Automatic Configuration Persistence**: The dynamic ngrok URL is now automatically persisted to the .env file for consistency.

### Changed
- **Generic Environment Management**: Refactored .env update logic in config.go to be more flexible and reusable across different settings.

## [1.1.0] - 2026-05-02

### Added
- **Pairing Persistence Helper**: Implemented SaveClientKey(newKey string) in config.go to safely persist the TV pairing key to the .env file while preserving other settings.

### Improved
- **Automatic Re-pairing**: Integrated automatic client key updates across all Telegram bot handlers (/tvstart, /tvstop, /tvnotify). The bot now seamlessly handles and persists new pairing keys if the TV resets its authorization.

### Removed
- **Dead Code Cleanup**: Removed the obsolete testTV function and unused imports from tv.go, streamlining the codebase.

## [1.0.1] - 2026-05-01

**Summary of the 401 Fix:**
The original 401 insufficient permissions error was caused by the legacy go-webos library's inability to request a permission manifest during authorization. Modern LG WebOS versions (4.0+) require explicit manifests to grant access to sensitive services like notifications and channel lists. By implementing a custom, manifest-aware WebOS client that handles the WebSocket protocol directly, we've enabled full access to these services.

### Optimized Power Handlers
- **Optimized Power Handlers**:
    - Implemented IsRunning utility to detect TV state via TCP port 3001 polling.
    - Updated /tvstart to skip Wake-on-LAN and boot delays if the TV is already active.
    - Updated /tvstop to provide immediate "already off" feedback when the TV is unreachable, eliminating connection timeouts.

### Improved UX
- **Improved UX**: Added real-time progress notifications during the TV boot sequence.

### Fixed
- **401 Insufficient Permissions**: Resolved unauthorized access errors when controlling the LG WebOS TV.
    - **Root Cause**: The legacy go-webos library failed to provide a permission manifest during registration, leading to restricted access on modern WebOS versions.
    - **Solution**: Implemented a custom, manifest-aware WebOS client in tv.go using raw WebSockets.
    - **Permissions Added**: Included scopes for WRITE_NOTIFICATION_TOAST, CONTROL_AUDIO, READ_TV_CHANNEL_LIST, and READ_INSTALLED_APPS.

### Added
- **Custom WebOS Client**: A new WebOSTV struct with support for:
    - Manual manifest-based registration.
    - Mute(bool) helper for audio control.
    - Notification(string) helper for toast messages.
    - KeyExit() method to simulate the remote EXIT button press.
        - Implements secondary WebSocket connection to com.webos.service.networkinput.
        - Sends raw button events for remote parity with PyWebOSTV.
    - Generic Call(uri, payload) method for extensible API interaction.
- **Start Method**: Integrated TV wake-up and initial setup sequence.
    - Automates WakeTV (WoL) and waits 10 seconds for the OS to boot.
    - Automatically executes KeyExit to clear the boot-up overlays or home screen.
- **Environment Management**: Improved .env handling to persist the client_id across sessions automatically.
- **Added Stop, ChannelList, and SetVolume Methods**:
    - Stop(): Shuts down the TV using ssap://system/turnOff.
    - ChannelList(): Retrieves the current channel list via ssap://tv/getChannelList.
    - SetVolume(int): Controls the audio level using ssap://audio/setVolume.

- **Telegram Bot Integration**:
    - Added /tvstart command handler to remotely turn on the TV.
        - Full automation of the TV power-on sequence via Telegram.
        - Triggers Wake-on-LAN, handles the 10-second boot delay, and clears the home screen using KeyExit.
    - Added /tvstop command handler to remotely turn off the TV.
    - Added /tvnotify command: Enables sending custom toast notifications to the TV directly from Telegram.
    - Switched application entry point in main.go to launch the bot service.
    - Implemented secure authorization flow within bot handlers using cached client keys.

### Changed
- Refactored testTV in tv.go to demonstrate working notifications and audio controls.

- **Centralized Configuration**:
    - Created config.go to manage TV metadata (IP, MAC, Port) dynamically.
    - Moved all hardcoded TV identifiers to the .env file.
    - Refactored tv.go, tv-wake.go, and bot.go to use the new configuration system.
    - Included NGROK_URL in the centralized configuration system.
