package main

import (
	"net"
	//"encoding/xml"
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
	goxmpp.RegisterGlobalStreamFeatures()

	err := C2sServer()
	if err != nil { println(err.Error()) }
}



func C2sConnection(conn net.Conn) error {
	println("New connection")
	defer conn.Close()

	sw := goxmpp.NewStreamWrapper(conn)

	stream, err := sw.ReadStreamOpen()
	if err != nil { return err }
	stream.From, stream.To = stream.To, ""
	sw.WriteStreamOpen(stream, "jabber:client")

	println("** Received stream to:", stream.From)

	for {
		sw.Encoder.Encode(goxmpp.GlobalStreamFeatures.ExposeTo(sw))
		sw.Decoder.Token()
		sw.Decoder.Skip()
	}

	/*
	var features goxmpp.Features
	features.StartTLS = nil //new(goxmpp.StartTLS)
	features.Mechanisms = new(goxmpp.Mechanisms)
	features.Mechanisms.Names = append(features.Mechanisms.Names, "PLAIN", "DIGEST-MD5")
	sw.Encoder.Encode(features)

	mechanisms := map[[2]string](func(xml.StartElement) interface{}){
		[2]string{"auth", "urn:ietf:params:xml:ns:xmpp-sasl"}: func(e xml.StartElement) interface{} {
			// Look up e.attr[mechanism] to find the mechanism they want
			return new(goxmpp.DigestMD5Auth)
		},
	}

	c, err := sw.ReadXMLChunk(mechanisms)
	if err == nil {
		switch c := c.(type) {
		case *goxmpp.PlainAuth:
			println("** PLAIN:", c.Nonce)
		case *goxmpp.DigestMD5Auth:
			println("** MD5:", c.ID)
		}
	} else {
		println(err.Error())
	}

	// Just kidding...
	sw.RW.Write([]byte("<success xmlns='urn:ietf:params:xml:ns:xmpp-sasl'/>"))*/

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
	return nil
}
