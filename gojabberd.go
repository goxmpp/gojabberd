package main

import (
	"net"
	"encoding/xml"
	"github.com/dotdoom/goxmpp"
)

type C2s struct {
	Conn net.Conn
	Authenticated bool
}

var clients map[string]C2s

func C2sServer() error {
	listener, err := net.Listen("tcp", "0.0.0.0:5222")
	if err != nil { return err }

	for {
		conn, err := listener.Accept()
		if err == nil {
			go C2sConnection(conn)
		} else {
			println(err.Error());
		}
	}
}

func main() {
	err := C2sServer()
	if err != nil { println(err.Error()) }
}

func C2sConnection(conn net.Conn) error {
	println("New connection")
	sw := goxmpp.NewStreamWrapper(conn)

	stream, err := sw.ReadStreamOpen()
	if err != nil { return err }
	stream.From, stream.To = stream.To, ""
	sw.WriteStreamOpen(stream, "jabber:client")

	println("** Received stream to:", stream.From)

	var features goxmpp.Features
	features.StartTLS = nil //new(goxmpp.StartTLS)
	features.Mechanisms = new(goxmpp.Mechanisms)
	features.Mechanisms.Names = append(features.Mechanisms.Names, "DIGEST-MD5")
	sw.Encoder.Encode(features)

	mechanisms := map[[2]string](func(xml.StartElement) interface{}){
		[2]string{"auth", "urn:ietf:params:xml:ns:xmpp-sasl"}: func(xml.StartElement) interface{} { return new(goxmpp.DigestMD5Auth) },
	}

	md5, err := sw.ReadXMLChunk(mechanisms)
	if err == nil {
		println("** Received digest-md5 auth with id:", md5.(*goxmpp.DigestMD5Auth).ID)
	} else {
		println(err.Error())
	}

	// Just kidding...
	sw.RW.Write([]byte("<success xmlns='urn:ietf:params:xml:ns:xmpp-sasl'/>"))

/*
<stream:features>
	<starttls xmlns="urn:ietf:params:xml:ns:xmpp-tls"/>
	<compression xmlns="http://jabber.org/features/compress">
		<method>zlib</method>
	</compression>
	<mechanisms xmlns="urn:ietf:params:xml:ns:xmpp-sasl">
		<mechanism>PLAIN</mechanism>
		<mechanism>DIGEST-MD5</mechanism>
		<mechanism>SCRAM-SHA-1</mechanism>
	</mechanisms>
	<c xmlns="http://jabber.org/protocol/caps" node="http://www.process-one.net/en/ejabberd/" ver="rvAR01fKsc40hT0hOLGDuG25y9o=" hash="sha-1"/>
	<register xmlns="http://jabber.org/features/iq-register"/>
</stream:features>
*/

	println("Closing connection");
	return conn.Close()
}
