package main

import (
	"log"
	"vmesshub"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Ltime | log.Lshortfile)
	socks5ListenUri := "127.0.0.1:9000"

	log.Println("Local Socks5 :", socks5ListenUri)
	uuid := "uuid"
	remoteHost := "remote tcp host:port"
	securityType := "none"
	alterId := 64

	vh, err := vmesshub.CreateVmessHub(socks5ListenUri, remoteHost, uuid, securityType, alterId)

	if err != nil {
		log.Println(err)
		return
	}

	vh.StartSocks5Listen()
}
