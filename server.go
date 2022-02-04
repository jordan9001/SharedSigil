package main

import (
	"crypto/rand"
	"flag"
	"log"
	"math"
	"math/big"
	"net/http"
	"sync"
	"time"
)

type userConfig struct {
	clr             string
	ink             float32
	depth           float32
	centered        uint
	bristles        uint
	smoothing       float32
	lift_smoothing  float32
	start_smoothing float32
}

type dotsConfig struct {
	clr     string
	points  uint
	d       float32
	rp      float32
	pointup bool
}

type roomConfig struct {
	bg   string
	dots []dotsConfig
}

type userInfo struct {
	uid       uint32
	submitted bool
	conf      userConfig
}

type roomInfo struct {
	id    uint32
	exp   time.Time
	users []userInfo
	conf  roomConfig
	file  string
}

var roomsLock sync.RWMutex
var rooms map[uint32]roomInfo

// get_config: give a unique identifier and get back room config
func getConfig(w http.ResponseWriter, r *http.Request) {

}

// send_strokes: sends in completed drawing
func sendStrokes(w http.ResponseWriter, r *http.Request) {

}

// get_done: get back x/total submitted for your room, poll this
func getDone(w http.ResponseWriter, r *http.Request) {

}

// get_room: get current completed drawing
func getRoom(w http.ResponseWriter, r *http.Request) {

}

// create_room: create a room for x people and returns links (used in beginning)
func createRoom(w http.ResponseWriter, r *http.Request) {
	// get how many players

	// generate the room info
	var rinf roomInfo
	//TODO

	maxid := big.NewInt(math.MaxUint32)
	roomsLock.Lock()
	for {
		id, err := rand.Int(rand.Reader, maxid)
		if err != nil {
			log.Panicf("Could not generate random id! %v", err)
		}

		id32 := uint32(id.Uint64())

		_, ok := rooms[id32]
		if !ok {
			rooms[id32] = rinf
			break
		}
		// else continue to gen numbers till we find one
	}
	roomsLock.Unlock()
}

func serveRoom(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./site/sigl.html")
}

func main() {
	var port = flag.String("port", "10987", "Port for sigil server")
	var imgdir = flag.String("dir", "./", "Path to image directory")

	flag.Parse()

	rooms = make(map[uint32]roomInfo)

	log.Printf("Starting up sigl server on port %v @ %v", *port, *imgdir)

	fileServer := http.FileServer(http.Dir("site"))
	http.Handle("/", fileServer)
	http.HandleFunc("/s/", serveRoom)
	sigilServer := http.FileServer(http.Dir(*imgdir))
	http.Handle("/sigils/", sigilServer)
	http.HandleFunc("/api/get_config", getConfig)
	http.HandleFunc("/api/send_strokes", sendStrokes)
	http.HandleFunc("/api/get_done", getDone)
	http.HandleFunc("/api/get_room", getRoom)
	http.HandleFunc("/api/create_room", createRoom)

	// start goroutine to clean up timed-out rooms

	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
