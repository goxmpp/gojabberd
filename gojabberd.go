package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	_ "github.com/dotdoom/goxmpp"
	"github.com/dotdoom/goxmpp/extensions/features/auth"
	"github.com/dotdoom/goxmpp/extensions/features/auth/mechanisms/md5"
	"github.com/dotdoom/goxmpp/extensions/features/auth/mechanisms/plain"
	"github.com/dotdoom/goxmpp/extensions/features/auth/mechanisms/sha1"
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

var plain_auth = flag.Bool("plain", false, "Use PLAIN auth")
var md5_auth = flag.Bool("md5", false, "Use DigestMD5 auth")
var sha1_auth = flag.Bool("sha1", false, "Use SCRAM-SHA-1 auth")
var tls = flag.Bool("tls", false, "Use TLS")

// TODO path should be changed to something meaningful
var pem = flag.String("pem", "test/gojabberd.pem", "Path to pem file")
var key = flag.String("key", "test/gojabberd.key", "Path to key file")

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

	if *plain_auth {
		st.State.Push(&plain.PlainState{
			VerifyUserAndPassword: func(user string, password string) bool {
				fmt.Println("VerifyUserAndPassword (using PLAIN) for", user)
				return true
			},
			RequireEncryption: true,
		})
	}

	if *md5_auth {
		st.State.Push(&md5.DigestMD5State{
			ValidateMD5: func(c *md5.Challenge, r *md5.Response) bool {
				fmt.Println("Validating clinet's reply on our chalenge")

				// Test is a password which we should get from some where else
				password := "test"
				hash := r.GenerateHash(c, password)

				log.Println("Expected", hash, "Got", r.Response)
				return hash == r.Response
			},
			Realm: []string{"gojabberd"},
		})
	}

	if *sha1_auth {
		st.State.Push(&auth.AuthState{
			GetPasswordByUserName: func(username string) string {
				return "test"
			},
		})
		st.State.Push(&sha1.SHAState{})
	}

	st.State.Push(compression.NewCompressState())
	st.State.Push(starttls.NewStartTLSState(*tls, starttls.NewTLSConfig(*pem, *key)))

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
