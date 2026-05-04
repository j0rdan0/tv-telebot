package tv

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"telegram-bot/internal/config"

	"github.com/gorilla/websocket"
)

const (
	URISetMute           = "ssap://audio/setMute"
	URICreateToast       = "ssap://system.notifications/createToast"
	URITurnOff           = "ssap://system/turnOff"
	URIGetChannelList    = "ssap://tv/getChannelList"
	URIGetCurrentChannel = "ssap://tv/getCurrentChannel"
	URIGetPowerState     = "ssap://system/getPowerState"
	URIOpenChannel       = "ssap://tv/openChannel"
	URISetVolume         = "ssap://audio/setVolume"
	URIGetInputSocket    = "ssap://com.webos.service.networkinput/getPointerInputSocket"
)

// WebOSTV represents a connection to an LG WebOS TV.
type WebOSTV struct {
	conn *websocket.Conn
	mu   sync.Mutex
	id   int
}

// webosMessage represents the JSON structure for WebOS API messages.
type webosMessage struct {
	Type    string      `json:"type"`
	ID      string      `json:"id"`
	URI     string      `json:"uri,omitempty"`
	Payload interface{} `json:"payload,omitempty"`
}

// NewWebOSTV creates a new connection to a WebOS TV using the centralized config.
func NewWebOSTV() (*WebOSTV, error) {
	cfg := config.LoadConfig()
	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		NetDial:         (&net.Dialer{Timeout: time.Second * 5}).Dial,
	}
	// WebOS TVs use port 3001 for SSL/TLS encrypted WebSocket connections by default.
	conn, _, err := dialer.Dial("wss://"+cfg.TVIP+":"+cfg.TVPort, nil)
	if err != nil {
		return nil, err
	}
	return &WebOSTV{conn: conn}, nil
}

// Close closes the underlying WebSocket connection.
func (tv *WebOSTV) Close() error {
	if tv.conn != nil {
		return tv.conn.Close()
	}
	return nil
}

// Authorize performs the registration/handshake with the TV.
func (tv *WebOSTV) Authorize(key string) (string, error) {
	tv.mu.Lock()
	tv.id++
	id := fmt.Sprintf("%d", tv.id)
	tv.mu.Unlock()

	permissions := []string{
		"LAUNCH", "CONTROL_AUDIO", "CONTROL_POWER", "READ_INSTALLED_APPS",
		"CONTROL_DISPLAY", "READ_TV_CHANNEL", "READ_NETWORK_STATE",
		"CONTROL_TV_SCREEN", "READ_TERMINAL_CONTROL", "READ_LGE_SDP_SERVER_INFO",
		"READ_CURRENT_CHANNEL", "READ_CHANNEL_LIST", "READ_PROGRAM_INFO",
		"CONTROL_INPUT_TEXT", "CONTROL_MOUSE_AND_KEYBOARD", "WRITE_NOTIFICATION_TOAST",
		"READ_INPUT_DEVICE_LIST", "READ_TV_CHANNEL_LIST",
	}

	payload := map[string]interface{}{
		"forcePairing": false,
		"pairingType":  "PROMPT",
		"manifest": map[string]interface{}{
			"appId":    "com.gemini.bot",
			"vendorId": "google",
			"localizedAppNames": map[string]string{
				"": "Gemini Bot",
			},
			"permissions": permissions,
		},
	}
	if key != "" {
		payload["client-key"] = key
	}

	msg := webosMessage{
		Type:    "register",
		ID:      id,
		Payload: payload,
	}

	if err := tv.conn.WriteJSON(msg); err != nil {
		return "", err
	}

	for {
		_, raw, err := tv.conn.ReadMessage()
		if err != nil {
			return "", err
		}
		var resp map[string]interface{}
		json.Unmarshal(raw, &resp)

		if resp["type"] == "registered" {
			p := resp["payload"].(map[string]interface{})
			return p["client-key"].(string), nil
		} else if resp["type"] == "error" {
			return "", fmt.Errorf("auth error: %v", resp["error"])
		}
	}
}

// Call sends a request to a specific WebOS API URI with an optional payload.
func (tv *WebOSTV) Call(uri string, payload interface{}) (map[string]interface{}, error) {
	tv.mu.Lock()
	tv.id++
	id := fmt.Sprintf("%d", tv.id)
	tv.mu.Unlock()

	msg := webosMessage{
		Type:    "request",
		ID:      id,
		URI:     uri,
		Payload: payload,
	}

	if err := tv.conn.WriteJSON(msg); err != nil {
		return nil, err
	}

	for {
		_, raw, err := tv.conn.ReadMessage()
		if err != nil {
			return nil, err
		}
		var resp map[string]interface{}
		json.Unmarshal(raw, &resp)

		if resp["id"] == id {
			if resp["type"] == "error" {
				return nil, fmt.Errorf("api error: %v", resp["error"])
			}
			return resp["payload"].(map[string]interface{}), nil
		}
	}
}

// Mute sets the mute state of the TV.
func (tv *WebOSTV) Mute(mute bool) error {
	_, err := tv.Call(URISetMute, map[string]interface{}{
		"mute": mute,
	})
	return err
}

// Notification displays a toast notification on the TV.
func (tv *WebOSTV) Notification(message string) error {
	_, err := tv.Call(URICreateToast, map[string]interface{}{
		"message": message,
	})
	return err
}

// Stop turns the TV off.
func (tv *WebOSTV) Stop() error {
	_, err := tv.Call(URITurnOff, nil)
	return err
}

// ChannelList gets the channel list.
func (tv *WebOSTV) ChannelList() (map[string]interface{}, error) {
	return tv.Call(URIGetChannelList, nil)
}

// GetCurrentChannel gets the currently active channel.
func (tv *WebOSTV) GetCurrentChannel() (map[string]interface{}, error) {
	return tv.Call(URIGetCurrentChannel, nil)
}

// GetPowerState gets the current power state of the TV.
func (tv *WebOSTV) GetPowerState() (string, error) {
	resp, err := tv.Call(URIGetPowerState, nil)
	if err != nil {
		return "", err
	}
	if state, ok := resp["state"].(string); ok {
		return state, nil
	}
	return "unknown", nil
}

// SetChannel changes the channel by its unique ID.
func (tv *WebOSTV) SetChannel(channelId string) error {
	_, err := tv.Call(URIOpenChannel, map[string]interface{}{
		"channelId": channelId,
	})
	return err
}

// SetVolume sets the volume level.
func (tv *WebOSTV) SetVolume(level int) error {
	_, err := tv.Call(URISetVolume, map[string]interface{}{
		"volume": level,
	})
	return err
}

// StartTV turns on the TV using WoL, waits for it to boot, and returns a connected client.
func StartTV() (*WebOSTV, error) {
	WakeTV()
	fmt.Println("Waiting for TV to boot...")
	time.Sleep(30 * time.Second)

	tv, err := NewWebOSTV()
	if err != nil {
		return nil, fmt.Errorf("failed to connect after wake: %v", err)
	}
	return tv, nil
}

// KeyExit simulates pressing the EXIT button on the remote.
func (tv *WebOSTV) KeyExit() error {
	// 1. Get the pointer input socket
	resp, err := tv.Call(URIGetInputSocket, nil)
	if err != nil {
		return fmt.Errorf("failed to get input socket: %v", err)
	}

	socketPath, ok := resp["socketPath"].(string)
	if !ok {
		return fmt.Errorf("socketPath not found in response")
	}

	// 2. Connect to the input socket
	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	conn, _, err := dialer.Dial(socketPath, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to input socket: %v", err)
	}
	defer conn.Close()

	// 3. Send the EXIT button command
	// Format: "type:button\nname:EXIT\n\n"
	message := "type:button\nname:EXIT\n\n"
	err = conn.WriteMessage(websocket.TextMessage, []byte(message))
	if err != nil {
		return fmt.Errorf("failed to send EXIT button: %v", err)
	}

	return nil
}

// IsRunning checks if the TV's API service is reachable on the network.
func IsRunning() bool {
	cfg := config.LoadConfig()
	address := net.JoinHostPort(cfg.TVIP, cfg.TVPort)
	conn, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
