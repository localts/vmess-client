package vmesshub

import (
	"encoding/binary"
	"errors"
	"ext"
	"fmt"
	"io"
	"log"
	"net"
	"runtime/debug"
	"socks5"
	"strings"
	"time"
	"vmess"
)

const (
	REMOTE_DAIL_TIMEOUT    = 10
	CONN_M0AX_LIVE_TIMEOUT = 60
	BUFFER_SIZE            = 256
)

type VmessHub struct {
	S5         *socks5.Socks5
	VC         *vmess.Client
	RemoteHost string
}

func CreateVmessHub(socksUri string, remoteHost string, uuid string, securityType string, alterId int) (vh *VmessHub, err error) {
	if len(remoteHost) == 0 || len(uuid) == 0 {
		err = errors.New("Vmess client information is incomplete.")
		return
	}

	vh = &VmessHub{RemoteHost: strings.Trim(remoteHost, " ")}

	vh.S5, err = socks5.GetNewSocks5(socksUri)

	if err != nil {
		return
	}

	vh.VC, err = vmess.NewClient(uuid, securityType, alterId)

	if err != nil {
		return
	}

	return
}

func (vh *VmessHub) StartSocks5Listen() {
	for {
		client, err := vh.S5.Listener.Accept()
		if err != nil {
			break
		}

		go vh.handleLocalClient(client)
	}
}

func (vh *VmessHub) handleLocalClient(client net.Conn) (err error) {

	defer func() {
		client.Close()
	}()

	//	log.Printf("connected from %v.", client.RemoteAddr())

	data := make([]byte, 1)
	n, err := client.Read(data)

	if err != nil || n != 1 {
		log.Println(err)
		return
	}

	if data[0] == 5 {
		//log.Println("handle with socks v5")
		err = vh.handleSocks5Data(client)
		if err != nil {
			log.Println(err)
			return
		}
	} else if data[0] > 5 {
		//verbose("handle with http")
		//handleHTTP(client, data[0])
		log.Println("Error: only support Socksv5")

	} else {
		log.Println("Error: only support Socksv5")
	}

	return
}

func (vh *VmessHub) handleSocks5Data(client net.Conn) (err error) {

	buffer := make([]byte, 1) //read NMETHODS

	_, err = io.ReadFull(client, buffer)

	if err != nil {
		log.Println("cannot read from client")
		return
	}

	buffer = make([]byte, buffer[0]) //read METHODS

	_, err = io.ReadFull(client, buffer)

	if err != nil {
		log.Println("cannot read from client")
		return
	}

	if !ext.ByteInArray(0, buffer) { //sock5 No certification required
		log.Println("client not support bare connect")
		return
	}

	// send initial SOCKS5 response (VER, METHOD)
	client.Write([]byte{5, 0})

	buffer = make([]byte, 4)
	_, err = io.ReadFull(client, buffer)

	if err != nil {
		log.Println("failed to read (ver, cmd, rsv, atyp) from client")
		return
	}

	ver, cmd, atyp := buffer[0], buffer[1], buffer[3]

	if ver != 5 {
		log.Println("ver should be 5, got %v", ver)
		return
	}

	// 1: connect 2: bind
	if cmd != 1 && cmd != 2 {
		log.Println("bad cmd:%v", cmd)
		return
	}

	shost := ""
	sport := ""

	if atyp == socks5.ATYP_IPV6 {

		log.Println("do not support ipv6 yet")
		return

	} else if atyp == socks5.ATYP_DOMAIN {

		buffer = make([]byte, 1) //read domain len

		_, err = io.ReadFull(client, buffer)

		if err != nil {
			log.Println("cannot read from client")
			return
		}

		buffer = make([]byte, buffer[0]) //read domain

		_, err = io.ReadFull(client, buffer)

		if err != nil {
			log.Println("cannot read from client")
			return
		}

		shost = string(buffer)

	} else if atyp == socks5.ATYP_IPV4 {

		buffer = make([]byte, 4) // read ip

		_, err = io.ReadFull(client, buffer)

		if err != nil {
			log.Println("cannot read from client")
			return
		}

		shost = net.IP(buffer).String()

	} else {

		log.Println("bad atyp: %v", atyp)
		return

	}

	buffer = make([]byte, 2) // read port

	_, err = io.ReadFull(client, buffer)

	if err != nil {
		log.Println("cannot read port from client")
		return
	}

	sport = fmt.Sprintf("%d", binary.BigEndian.Uint16(buffer))

	// reply to client to estanblish the socks v5 connection
	client.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})

	//	log.Println(shost, sport)

	err = vh.handleRemote(client, shost, sport, vh.RemoteHost)

	if err != nil {
		//log.Println(err)
		return
	}
	return
}

func (vh *VmessHub) handleRemote(localConn net.Conn, shost, sport, rhost string) (err error) {

	remoteConn, err := net.DialTimeout("tcp", rhost, time.Second*REMOTE_DAIL_TIMEOUT)

	if err != nil {
		return
	}

	defer func() {
		debug.FreeOSMemory()
		remoteConn.Close()
	}()

	vcRemoteConn, err := vh.VC.NewConn(remoteConn, fmt.Sprintf("%s:%s", shost, sport))

	if err != nil {
		return
	}

	ch_client := make(chan byte)
	ch_remote := make(chan byte)

	go readDataFromClient(ch_client, localConn, vcRemoteConn)
	go readDataFromRemote(ch_remote, vcRemoteConn, localConn)

	for {

		select {

		case _, ok := <-ch_remote:
			if !ok {
				return
			}
		case _, ok := <-ch_client:
			if !ok {
				return
			}
		case <-time.After(time.Second * time.Duration(CONN_M0AX_LIVE_TIMEOUT)):
//			log.Printf("close conn by timer %s:%s\n", shost, sport)
			return
		}

	}

	return
}

func readDataFromClient(ch chan byte, localConn net.Conn, remoteConn net.Conn) {
	bufferData := make([]byte, BUFFER_SIZE)

	for {
		localConn.SetReadDeadline(time.Now().Add(time.Second * 5))
		n, err := localConn.Read(bufferData)
		if err != nil {
			//log.Println(err)
			break
		}
		localConn.SetWriteDeadline(time.Now().Add(time.Second * 5))
		_, err = remoteConn.Write(bufferData[:n])

		if err != nil {
			break
		}
	}
	close(ch)
}

func readDataFromRemote(ch chan byte, remoteConn net.Conn, localConn net.Conn) {
	defer func() {
		//log.Println("leave readDataFromServer")
	}()
	data := make([]byte, BUFFER_SIZE)
	for {
		remoteConn.SetReadDeadline(time.Now().Add(time.Second * 5))
		n, err := remoteConn.Read(data)
		if err != nil {
			break
		}
		remoteConn.SetWriteDeadline(time.Now().Add(time.Second * 5))
		_, err = localConn.Write(data[:n])
		if err != nil {
			break
		}

	}
	close(ch)
}
