package socks5

import (
	"errors"
	"net"
	"vmess"
)

const (
	ATYP_IPV4   = 1
	ATYP_DOMAIN = 3
	ATYP_IPV6   = 4
)

type Socks5 struct {
	ListenUri string
	Listener  net.Listener
	VC        *vmess.Client
}

func GetNewSocks5(listenUri string) (s5 *Socks5, err error) {
	if len(listenUri) == 0 {
		err = errors.New("Socks5 listen uri is empty.")
		return
	}

	s5 = &Socks5{ListenUri: listenUri}

	s5.Listener, err = net.Listen("tcp", s5.ListenUri)
	if err != nil {
		return
	}

	return
}

