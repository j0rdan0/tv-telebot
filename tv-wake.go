package main

import (
	"fmt"
	"log"

	"github.com/ahiggins0/go-wol"
)

func WakeTV() {
	tvMac := LoadConfig().TVMac
	err := wol.SendMagicPacket(tvMac, "", "")
	if err != nil {
		log.Println("failed sending magic packet, err: ", err)
		return
	}
	fmt.Println("TV started")
}
