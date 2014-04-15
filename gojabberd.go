package main

import (
	"flag"
	"fmt"
	"net"

	_ "github.com/dotdoom/goxmpp"
	"github.com/dotdoom/goxmpp/extensions/features/auth/mechanisms"
	"github.com/dotdoom/goxmpp/extensions/features/bind"
	"github.com/dotdoom/goxmpp/extensions/features/compression"
	"github.com/dotdoom/goxmpp/extensions/features/starttls"
	"github.com/dotdoom/goxmpp/stream"
	"github.com/dotdoom/goxmpp/stream/elements/features"
	"github.com/dotdoom/goxmpp/stream/elements/stanzas/presence"
)

/*type C2s struct {
	Conn          net.Conn
	Authenticated bool
}

var clients map[string]C2s*/

var tls = flag.Bool("tls", false, "Use TLS")

// TODO path should be changed to something meaningful
var pem = flag.String("pem", "test/cert.pem", "Path to pem file")
var key = flag.String("key", "test/cert.key", "Path to key file")

func C2sServer() error {
	listener, err := net.Listen("tcp", "0.0.0.0:5222")
	if err != nil {
		return err
	}

	println("Server started")
	for {
		conn, err := listener.Accept()
		if err == nil {
			go C2sConnection(conn)
		} else {
			println(err.Error())
		}
	}
}

func main() {
	flag.Parse()
	err := C2sServer()
	if err != nil {
		println(err.Error())
	}
}

func C2sConnection(conn net.Conn) error {
	println("New connection")
	var st *stream.Stream

	st = stream.NewStream(conn)
	st.DefaultNamespace = "jabber:client"

	// Push states for all features we want to use
	//st.State.Push(&methods.GzipState{Level: 5})

	st.State.Push(&bind.BindState{
		VerifyResource: func(rc string) bool {
			fmt.Println("Using resource", rc)
			return true
		},
	})

	st.State.Push(&mechanisms.PlainState{
		VerifyUserAndPassword: func(user string, password string) bool {
			fmt.Println("VerifyUserAndPassword (using PLAIN) for", user)
			return true
		},
		RequireEncryption: true,
	})

	st.State.Push(compression.NewCompressState())
	if *tls {
		st.State.Push(starttls.NewStartTLSState(starttls.NewTLSConfig(*pem, *key)))
	}

	/*st.State.Push(&mechanisms.DigestMD5State{Callback: func(user string, salt string) string {
		fmt.Println("Trying to auth (using DIGEST-MD5)", user)
		return salt
	}})*/

	return st.Open(func(s *stream.Stream) error {
		// Go through the features loop until stream is finally open (or something wrong happens)
		if err := features.Loop(st); err != nil {
			fmt.Println("Features loop failed.", err)
			return err
		}

		fmt.Println("gojabberd: stream opened, required features passed. JID is", st.To)

		pr := presence.NewPresenceElement()
		pr.From = "test@localhost"
		pr.To = st.To
		pr.Status = ""
		pr.Show = "I'm online!"
		st.WriteElement(pr)

		for {
			e, err := st.ReadElement()
			if err != nil {
				fmt.Printf("gojabberd: cannot read element: %v\n", err)
				return err
			}
			fmt.Printf("gojabberd: received element: %#v\n", e)
			if feature_handler, ok := e.(features.Handler); ok {
				fmt.Println("gojabberd: calling feature handler")
				if err := feature_handler.Handle(st); err != nil {
					fmt.Printf("gojabberd: cannot handle feature: %v\n", err)
					continue
					//return err
				}
				fmt.Println("gojabberd: feature handler completed")
			} else {
				if stanza, ok := e.(*presence.PresenceElement); ok {
					fmt.Println("gojabberd: got stanza, responding")
					stanza.From = "localhost"
					stanza.To = st.To
					st.WriteElement(stanza)
				}
			}
		}

		return nil
	})
}
