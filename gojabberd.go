package main

import (
	"database/sql"
	"flag"
	"fmt"
	"net"

	_ "github.com/goxmpp/goxmpp"
	"github.com/goxmpp/goxmpp/extensions/features/auth"
	"github.com/goxmpp/goxmpp/extensions/features/auth/mechanisms/md5"
	"github.com/goxmpp/goxmpp/extensions/features/auth/mechanisms/plain"
	"github.com/goxmpp/goxmpp/extensions/features/auth/mechanisms/sha1"
	"github.com/goxmpp/goxmpp/extensions/features/bind"
	"github.com/goxmpp/goxmpp/extensions/features/compression"
	"github.com/goxmpp/goxmpp/extensions/features/starttls"
	"github.com/goxmpp/goxmpp/stream"
	"github.com/goxmpp/goxmpp/stream/features"
	"github.com/goxmpp/goxmpp/stream/stanzas/presence"
	"github.com/goxmpp/xtream"
	_ "github.com/mattn/go-sqlite3"
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

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}

	defer db.Close()

	if _, err := db.Exec("CREATE TABLE users (username VARCHAR(250) PRIMARY KEY, password VARCHAR(250))"); err != nil {
		panic(err)
	}
	if _, err := db.Exec("INSERT INTO users (username, password) VALUES (?, ?)", "user", "secret"); err != nil {
		panic(err)
	}

	println("Server started")
	for {
		conn, err := listener.Accept()
		if err == nil {
			go C2sConnection(conn, db)
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

func C2sConnection(conn net.Conn, db *sql.DB) error {
	println("New connection")
	var st *stream.Stream

	st = stream.NewStream(conn)

	for _, fe := range features.FeatureFactory.List() {
		feature := fe.Constructor(nil)
		st.ElementFactory.AddNamed(
			func() xtream.Element { return feature.InitHandler() },
			fe.Parent,
			fe.Name,
		)
		st.AddFeature(feature)
	}

	st.DefaultNamespace = "jabber:client"

	// Push states for all features we want to use
	//st.State.Push(&methods.GzipState{Level: 5})

	st.State.Push(&bind.BindState{
		VerifyResource: func(rc string) bool {
			fmt.Println("Using resource", rc)
			return true
		},
	})

	st.State.Push(&auth.AuthState{
		GetPasswordByUserName: func(username string) string {
			fmt.Println("gojabberd: GetPasswordByUserName for", username)
			var password string
			err := db.QueryRow("SELECT password FROM users WHERE username = ?", username).Scan(&password)
			switch {
			case err == sql.ErrNoRows:
				fmt.Println("gojabberd: no such user")
				return ""
			case err != nil:
				panic(err)
			default:
				return password
			}
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
			Realm: []string{"gojabberd"},
		})
	}

	if *sha1_auth {
		st.State.Push(&sha1.SHAState{})
	}

	st.State.Push(compression.NewCompressState())
	st.State.Push(starttls.NewStartTLSState(*tls, starttls.NewTLSConfig(*pem, *key)))

	/*st.State.Push(&mechanisms.DigestMD5State{Callback: func(user string, salt string) string {
		fmt.Println("Trying to auth (using DIGEST-MD5)", user)
		return salt
	}})*/

	return st.Open(func(s *stream.Stream) error {
		for s.HasRequired() {
			e, err := s.ReadElement()
			if err != nil {
				fmt.Printf("gojabberd: cannot read element: %v\n", err)
				return err
			}

			if handler, ok := e.(features.FeatureHandler); ok {
				if err := handler.Handle(s, nil); err != nil {
					fmt.Printf("gojabberd: error handling feature: %v\n", err)
					return err
				}
			} else {
				fmt.Printf("gojabberd: not a feature handler read while feature expected: %v\n", err)
			}

			if s.ReOpen {
				if err := s.ReadSentOpen(); err != nil {
					fmt.Printf("gojabberd: error reopening stream: %v\n", err)
					return err
				}
			}
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
			if feature_handler, ok := e.(features.FeatureHandler); ok {
				fmt.Println("gojabberd: calling feature handler")
				if err := feature_handler.Handle(st, nil); err != nil {
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
