package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"

	_ "github.com/goxmpp/goxmpp"
	"github.com/goxmpp/goxmpp/extensions/features/auth"

	"github.com/goxmpp/goxmpp/extensions/features/bind"
	"github.com/goxmpp/goxmpp/stream"
	"github.com/goxmpp/goxmpp/stream/features"
	"github.com/goxmpp/goxmpp/stream/stanzas/presence"
	_ "github.com/mattn/go-sqlite3"
)

/*type C2s struct {
	Conn          net.Conn
	Authenticated bool
}

var clients map[string]C2s*/

var config = []byte(`{
	"compression":{"zlib": {}, "lzw": {"Level": 6}},
	"auth":["SCRAM-SHA-1", "DIGEST-MD5"],
	"starttls":{
		"required":false,
		"pem":"test/gojabberd.pem",
		"key":"test/gojabberd.key"
	}
}`)

var db = initUserDB()

func initUserDB() *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}

	if _, err := db.Exec("CREATE TABLE users (username VARCHAR(250) PRIMARY KEY, password VARCHAR(250))"); err != nil {
		panic(err)
	}
	if _, err := db.Exec("INSERT INTO users (username, password) VALUES (?, ?)", "user", "secret"); err != nil {
		panic(err)
	}

	return db
}

func C2sServer(raw_config stream.RawConfig) error {
	listener, err := net.Listen("tcp", "0.0.0.0:5222")
	if err != nil {
		return err
	}

	println("Server started")
	for {
		conn, err := listener.Accept()
		if err == nil {
			go C2sConnection(conn, db, raw_config)
		} else {
			println(err.Error())
		}
	}
}

func main() {
	flag.Parse()
	var raw_config stream.RawConfig
	if err := json.Unmarshal(config, &raw_config); err != nil {
		log.Println("gojabberd error parsing config:", err)
	}

	err := C2sServer(raw_config)
	if err != nil {
		println(err.Error())
	}
}

func C2sConnection(conn net.Conn, db *sql.DB, rconf stream.RawConfig) error {
	println("New connection")

	st := stream.NewServerStream(conn, features.DependencyGraph, rconf)
	features.EnableStreamFeatures(st, "stream")

	st.SetDefaultNamespace("jabber:client")

	st.State().Push(&bind.BindState{
		VerifyResource: func(rc string) bool {
			fmt.Println("Using resource", rc)
			return true
		},
	})

	st.State().Push(&auth.AuthState{
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

	if err := st.Open(func(s stream.ServerStream) error {
		e, err := s.ReadElement()
		if err != nil {
			log.Println("gojabberd: cannot read element:", err)
			return err
		}

		if handler, ok := e.(features.FeatureHandler); ok {
			if err := handler.Handle(s, nil); err != nil {
				log.Println("gojabberd: error handling feature:", err)
				return err
			}
		} else {
			log.Println("gojabberd: not a feature handler read while feature expected:", err)
		}

		return nil
	}); err != nil {
		log.Println("gojabberd: could not open stream:", err)
		return err
	}

	log.Println("gojabberd: stream opened, required features passed. JID is", st.To)

	pr := presence.NewPresenceElement()
	pr.From = "test@localhost"
	pr.To = st.To()
	pr.Status = ""
	pr.Show = "I'm online!"
	st.WriteElement(pr)

	for st.Opened() {
		e, err := st.ReadElement()
		if err != nil {
			fmt.Printf("gojabberd: cannot read element: %v\n", err)
			return err
		}
		log.Printf("gojabberd: received element: %#v\n", e)
		if feature_handler, ok := e.(features.FeatureHandler); ok {
			log.Println("gojabberd: calling feature handler")
			if err := feature_handler.Handle(st, nil); err != nil {
				fmt.Printf("gojabberd: cannot handle feature: %v\n", err)
				continue
			}
			log.Println("gojabberd: feature handler completed")
		} else {
			if stanza, ok := e.(*presence.PresenceElement); ok {
				fmt.Println("gojabberd: got stanza, responding")
				stanza.From = "localhost"
				stanza.To = st.To()
				st.WriteElement(stanza)
			}
		}
	}

	return nil
}
