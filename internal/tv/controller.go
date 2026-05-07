package tv

import (
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"telegram-bot/internal/config"
)

var (
	ErrPowerOpInProgress = errors.New("a power operation is already in progress")
	ErrAlreadyOn        = errors.New("TV is already running")
	ErrAlreadyOff       = errors.New("TV is already off")
	ErrInStandby       = errors.New("TV is in standby")
)

type Controller struct {
	sharedTV   *WebOSTV
	sharedTVMu sync.Mutex
	powerMu    sync.Mutex
}

func NewController() *Controller {
	return &Controller{}
}

func (c *Controller) GetConnection() (*WebOSTV, error) {
	c.sharedTVMu.Lock()
	defer c.sharedTVMu.Unlock()

	if c.sharedTV != nil {
		return c.sharedTV, nil
	}

	webos, err := NewWebOSTV()
	if err != nil {
		return nil, err
	}

	key := os.Getenv("client_id")
	newKey, err := webos.Authorize(key)
	if err != nil {
		webos.Close()
		return nil, fmt.Errorf("authorization failed: %v", err)
	}

	if newKey != "" && newKey != key {
		_ = config.SaveClientKey(newKey)
	}

	c.sharedTV = webos
	return c.sharedTV, nil
}

func (c *Controller) ClearConnection() {
	c.sharedTVMu.Lock()
	defer c.sharedTVMu.Unlock()
	if c.sharedTV != nil {
		_ = c.sharedTV.Close()
		c.sharedTV = nil
	}
}

func (c *Controller) Start() error {
	if !c.powerMu.TryLock() {
		return ErrPowerOpInProgress
	}
	defer c.powerMu.Unlock()

	if !IsRunning() {
		return c.triggerWake()
	}

	webos, err := c.GetConnection()
	if err != nil {
		// Connection failed, treat as off
		return c.triggerWake()
	}

	state, _ := webos.GetPowerState()
	log.Printf("TV Power State for Start: %s", state)

	if state == "active" || state == "On" {
		return ErrAlreadyOn
	}

	return c.triggerWake()
}

func (c *Controller) Stop() error {
	if !c.powerMu.TryLock() {
		return ErrPowerOpInProgress
	}
	defer c.powerMu.Unlock()

	if !IsRunning() {
		return ErrAlreadyOff
	}

	webos, err := c.GetConnection()
	if err != nil {
		return ErrAlreadyOff
	}

	state, _ := webos.GetPowerState()
	log.Printf("TV Power State for Stop: %s", state)

	if state == "standby" || state == "Screen Off" {
		return ErrInStandby
	}

	err = webos.Stop()
	if err != nil {
		c.ClearConnection()
		// If the error is just that the connection closed, it usually means the TV is shutting down
		if err.Error() == "websocket: close sent" || err.Error() == "use of closed network connection" {
			return nil
		}
		return fmt.Errorf("failed to stop TV: %v", err)
	}

	c.ClearConnection()
	return nil
}

func (c *Controller) triggerWake() error {
	webos, err := StartTV()
	if err != nil {
		return fmt.Errorf("failed to start TV: %v", err)
	}

	key := os.Getenv("client_id")
	newKey, err := webos.Authorize(key)
	if err == nil && newKey != key {
		_ = config.SaveClientKey(newKey)
	}

	// Save to shared TV instance
	c.sharedTVMu.Lock()
	if c.sharedTV != nil {
		c.sharedTV.Close()
	}
	c.sharedTV = webos
	c.sharedTVMu.Unlock()

	_ = webos.KeyExit()
	return nil
}
