# Changelog

## [1.5.0] - 2026-05-02

### Fixed
- Channel Switching: Completely refactored the channel switching logic. The bot now automatically maps human-readable channel numbers to the internal WebOS channel IDs required by the API.
- Menu Visibility: Switched the control menu to HTML parse mode and added an explicit /start command to ensure all interactive buttons (including Set Channel and Back) are always visible and functional.
- Back Feature: Fixed the Back command to correctly store and restore the previous channel using stable channel IDs.

### Improved
- Persistent Menu: Registered all bot commands with Telegram's persistent command menu for easier access.

## [1.4.3] - 2026-05-02

### Fixed
- UI Consistency: Conducted a project-wide removal of emoticons from all user-facing strings and documentation.

## [1.4.2] - 2026-05-02

### Added
- Makefile: Added a Makefile to the root directory for standardizing build and run tasks.

## [1.4.1] - 2026-05-02

### Added
- Set Channel Button: Added a Set Channel button to the interactive control menu.

### Fixed
- Missing Menu Actions: Integrated the missing channel control actions into the interactive button handler.

## [1.4.0] - 2026-05-02

### Added
- Channel Navigation: Added commands and interactive buttons to change channels and navigate back.
- TV Client Enhancements: Added SetChannel, GetCurrentChannel, and Close methods to the WebOSTV struct.

### Improved
- State Management: The bot now tracks the last active channel per chat, enabling the Back functionality.

## [1.3.0] - 2026-05-02

### Changed
- Project Structure Refactor: Adopted the standard Go project layout.

## [1.2.4] - 2026-05-02

### Fixed
- Ngrok Service Authentication: Added support for NGROK_AUTHTOKEN in the .env file.

## [1.2.3] - 2026-05-02

### Improved
- Ngrok Debugging: Added stderr piping for the ngrok process.

## [1.2.2] - 2026-05-02

### Fixed
- Interactive Buttons: Resolved an issue where inline buttons in the control menu were not triggering TV actions.

## [1.2.1] - 2026-05-02

### Added
- Audio Control Handlers: Added /tvmute and /tvvolume commands to the Telegram bot.

## [1.2.0] - 2026-05-02

### Added
- Ngrok Automation: Created ngrok.go to automatically start an ngrok tunnel on port 8080.

## [1.1.0] - 2026-05-02

### Added
- Pairing Persistence Helper: Implemented SaveClientKey in config.go.

## [1.0.1] - 2026-05-01

**Summary of the 401 Fix:**
Implemented a custom, manifest-aware WebOS client to resolve authentication issues on modern LG TVs.
