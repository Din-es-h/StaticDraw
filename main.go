package main

import (
	"crypto/rand"
	"log"
	"math/big"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ---- message shapes ----

type inMsg struct {
	Type  string `json:"type"`
	Image string `json:"image,omitempty"`
}

type outMsg struct {
	Type     string `json:"type"`
	Role     string `json:"role,omitempty"`
	Room     string `json:"room,omitempty"`
	Seed     int64  `json:"seed,omitempty"`
	StartAt  int64  `json:"start_at,omitempty"`
	P1Image  string `json:"p1_image,omitempty"`
	P2Image  string `json:"p2_image,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// ---- room bookkeeping ----

type player struct {
	conn  *websocket.Conn
	role  string // "p1" or "p2"
	image string
	mu    sync.Mutex // guards writes to conn
}

type room struct {
	mu      sync.Mutex
	code    string
	players []*player
	seed    int64
	startAt int64
	closed  bool
}

var (
	roomsMu sync.Mutex
	rooms   = map[string]*room{}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 1 << 20, // images can be a few hundred KB base64
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func send(p *player, m outMsg) {
	p.mu.Lock()
	defer p.mu.Unlock()
	_ = p.conn.WriteJSON(m)
}

func randSeed() int64 {
	n, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return time.Now().UnixNano()
	}
	return n.Int64()
}

func getOrCreateRoom(code string) *room {
	roomsMu.Lock()
	defer roomsMu.Unlock()
	r, ok := rooms[code]
	if !ok {
		r = &room{code: code}
		rooms[code] = r
	}
	return r
}

func removeRoomIfEmpty(code string) {
	roomsMu.Lock()
	defer roomsMu.Unlock()
	if r, ok := rooms[code]; ok {
		r.mu.Lock()
		empty := len(r.players) == 0
		r.mu.Unlock()
		if empty {
			delete(rooms, code)
		}
	}
}

func handleWS(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("room")
	if code == "" {
		http.Error(w, "room code required", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade error:", err)
		return
	}
	defer conn.Close()

	rm := getOrCreateRoom(code)

	rm.mu.Lock()
	if len(rm.players) >= 2 {
		rm.mu.Unlock()
		_ = conn.WriteJSON(outMsg{Type: "error", Reason: "room full"})
		return
	}
	role := "p1"
	if len(rm.players) == 1 {
		role = "p2"
	}
	me := &player{conn: conn, role: role}
	rm.players = append(rm.players, me)
	numPlayers := len(rm.players)
	rm.mu.Unlock()

	send(me, outMsg{Type: "joined", Role: role, Room: code})

	if numPlayers == 1 {
		send(me, outMsg{Type: "waiting"})
	} else {
		// second player just joined -> start the match
		rm.mu.Lock()
		rm.seed = randSeed()
		rm.startAt = time.Now().Add(3 * time.Second).UnixMilli()
		seed, startAt := rm.seed, rm.startAt
		players := append([]*player{}, rm.players...)
		rm.mu.Unlock()

		for _, p := range players {
			send(p, outMsg{Type: "start", Seed: seed, StartAt: startAt})
		}
	}

	// read loop
	for {
		var msg inMsg
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}
		switch msg.Type {
		case "final_image":
			rm.mu.Lock()
			me.image = msg.Image
			bothIn := true
			for _, p := range rm.players {
				if p.image == "" {
					bothIn = false
				}
			}
			var p1img, p2img string
			var players []*player
			if bothIn {
				for _, p := range rm.players {
					if p.role == "p1" {
						p1img = p.image
					} else {
						p2img = p.image
					}
				}
				players = append([]*player{}, rm.players...)
			}
			rm.mu.Unlock()

			if bothIn {
				for _, p := range players {
					send(p, outMsg{Type: "reveal", P1Image: p1img, P2Image: p2img})
				}
			}
		}
	}

	// disconnect cleanup
	rm.mu.Lock()
	remaining := rm.players[:0]
	for _, p := range rm.players {
		if p != me {
			remaining = append(remaining, p)
		}
	}
	rm.players = remaining
	others := append([]*player{}, rm.players...)
	rm.mu.Unlock()

	for _, p := range others {
		send(p, outMsg{Type: "opponent_left"})
	}
	removeRoomIfEmpty(code)
}

func main() {
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)
	http.HandleFunc("/ws", handleWS)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Println("chaos-draw server listening on", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
