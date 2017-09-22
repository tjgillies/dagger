package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	bencode "github.com/jackpal/bencode-go"
	"golang.org/x/crypto/ed25519"
	"log"
	//  "github.com/funkygao/golib/dag"
	"encoding/hex"
	//  "errors"
	//   "github.com/deckarep/golang-set"
	"flag"
	//   "reflect"
	// "github.com/davecgh/go-spew/spew"
	"github.com/mitchellh/mapstructure"
)

type Node struct {
	Hash    string
	Data    map[string]interface{}
	Parents []string
}

type Edge struct {
	From *Node
	To   *Node
}

type Dag struct {
	Edges   []Edge
	Nodes   []Node
	PrivKey ed25519.PrivateKey
	PubKey  string
}

type Message struct {
	Author string
	Type   string
	Body   map[string]interface{}
}

func SliceIndex(limit int, predicate func(i int) bool) int {
	for i := 0; i < limit; i++ {
		if predicate(i) {
			return i
		}
	}
	return -1
}

func canonicalSign(data interface{}, priv ed25519.PrivateKey) []byte {
	var buf bytes.Buffer
	bencode.Marshal(&buf, data)
	return ed25519.Sign(priv, buf.Bytes())
}

func p(data interface{}) {
	fmt.Printf("%#v\n\n", data)
}

func ProcessNode(nodeInterface interface{}, dag *Dag, rm *RequestManager) {
	var nodes []Node
	err := mapstructure.Decode(nodeInterface, &nodes)
	if err != nil {
		log.Fatal(err)
	}
	for _, node := range nodes {
		if len(node.Parents) < 1 {
			dag.Nodes = append(dag.Nodes, node)
			dag.Attach(&node)
		} else {
			parentHash := node.Parents[0]
			parent, err := dag.GetNode(parentHash)
			if err != nil {
				p(err.Error())

				rm.NodeRequest(parentHash)
			} else {
				p(parent)
				dag.Nodes = append(dag.Nodes, node)
				dag.Attach(&node)
			}
		}
	}

}

func main() {
	isServer := flag.Bool("server", false, "run in server mode")
	isClient := flag.Bool("client", false, "run in client mode")
	flag.Parse()
	if *isServer && *isClient {
		log.Fatal("pick one")
	}

	seed := make([]byte, ed25519.PrivateKeySize)
	rand.Read(seed)
	reader := bytes.NewReader(seed)
	pub, priv, err := ed25519.GenerateKey(reader)
	if err != nil {
		log.Fatal(err)
	}

	dag := Dag{PrivKey: priv, PubKey: hex.EncodeToString(pub)}
	if *isServer {
		p("Starting server")
		dag.CreateRoom("test")
		dag.CreateMessage("test message")
		clientManager := startServer(&dag)
		clientManager.messages <- []byte("Testing")
		for {
		}
	} else {
		requests := make(chan []byte)
		responses := make(chan []byte)
		rm := RequestManager{Requests: make(map[string]ResponseHandler), RequestChannel: requests, Dag: &dag}
		go connectPeer(requests, responses)
		for {
			response := <-responses
			var responseMap map[string]interface{}
			json.Unmarshal(response, &responseMap)
			p("response map")
			p(responseMap)

			if responseMap["result"] != nil {
				responseId := responseMap["id"].(string)
				handler := rm.Requests[responseId]
				p("handler")
				p(handler)
				p("processing node interface")
				handler.Function(responseMap["result"], &dag, &rm)
				p(dag)
			}

			if responseMap["method"] == "addNode" {
				p("ADD NODE")
				ProcessNode(responseMap["params"], &dag, &rm)
				p(dag)
			}
		}
	}
}