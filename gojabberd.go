package main

import (
	"fmt"
	"net"

	_ "github.com/dotdoom/goxmpp"
	"github.com/dotdoom/goxmpp/extensions/features/auth/mechanisms"
	"github.com/dotdoom/goxmpp/extensions/features/bind"
	"github.com/dotdoom/goxmpp/stream"
	"github.com/dotdoom/goxmpp/stream/elements/features"
)

/*type C2s struct {
	Conn          net.Conn
	Authenticated bool
}

var clients map[string]C2s*/

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
	err := C2sServer()
	if err != nil {
		println(err.Error())
	}
}

func C2sConnection(conn net.Conn) error {
	println("New connection")
	defer func() {
		conn.Close()
		println("Connection closed")
	}()

	st := stream.NewStream(conn)

	// Push states for all features we want to use
	//st.State.Push(&methods.GzipState{Level: 5})

	st.State.Push(&bind.BindState{
		VerifyResource: func(rc string) bool {
			fmt.Println("Using resource", rc)
			return true
		},
	})

	st.State.Push(&mechanisms.PlainState{
		Callback: func(user string, password string) bool {
			fmt.Println("Trying to auth (using PLAIN)", user)
			return true
		},
		RequireEncryption: true,
	})

	/*st.State.Push(&mechanisms.DigestMD5State{Callback: func(user string, salt string) string {
		fmt.Println("Trying to auth (using DIGEST-MD5)", user)
		return salt
	}})*/

	// Go through the features loop until stream is finally open (or something wrong happens)
	for st.Opened != true {
		st.ReadOpen()
		st.From, st.To = st.To, ""
		st.WriteOpen()

		if err := features.Loop(st); err != nil {
			fmt.Println("Features loop failed.", err)
			return err
		}
	}

	fmt.Println("Stream opened, required features passed. JID is", st.To)

	for {
		e, err := st.ReadElement()
		if err != nil {
			fmt.Printf("cannot read element: %v\n", err)
			return err
		}
		fmt.Printf("got element: %#v", e)
		if feature_handler, ok := e.(features.Handler); ok {
			fmt.Println("calling feature handler")
			if err := feature_handler.Handle(st); err != nil {
				fmt.Printf("cannot handle feature: %v\n", err)
				return err
			}
			fmt.Println("feature handler completed")
		}
	}

	return nil
}
