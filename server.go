package main

import (
	"crypto/rand"
	"flag"
	"log"
	"math"
	"math/big"
	"net/http"
	"os"
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
	id        uint32
	exp       time.Time
	users     []userInfo
	submitted int
	conf      roomConfig
	file      string
	flock     sync.Mutex
}

var roomsLock sync.RWMutex
var rooms map[uint32]roomInfo

// get_config: give a unique identifier and get back room config
func getConfig(w http.ResponseWriter, r *http.Request) {
	//TODO
	// lock rooms for reading
	// check the user exists
	// copy over config data
	// unlock rooms
	// generate a config for the user to return
	// respond
}

// send_strokes: sends in completed drawing
func sendStrokes(w http.ResponseWriter, r *http.Request) {
	var prevSubmit int

	//TODO
	// get drawing information sumbitted
	// lock the rooms mux for reading
	// lock the file mux

	// edit the new data
	// write the new data into a new file
	// replace the old file (so requests wont get half written files)

	// release the file mux
	// release the rooms mux
}

// get_done: get back x/total submitted for your room, polled
func getDone(w http.ResponseWriter, r *http.Request) {
	var id uint32

	var done int
	var outof int
	// get id from req

	roomsLock.RLock()

	_, ok := rooms[id]

	if ok {
		outof = len(rooms[id].users)
		done = rooms[id].submitted
	}

	roomsLock.RUnlock()

	//TODO respond based on ok and done, outof
}

// get_room: get current completed drawing
//TODO don't need this? Just have predictable file paths based on id?
func getRoom(w http.ResponseWriter, r *http.Request) {
	var id uint32
	var imgpath string

	// get id from req
	roomsLock.RLock()

	_, ok := rooms[id]

	if ok {
		imgpath = rooms[id].file
	}

	roomsLock.RUnlock()

	//TODO respond based on ok and imgpath
}

// create_room: create a room for x people and returns links (used in beginning)
func createRoom(w http.ResponseWriter, r *http.Request) {
	// get how many players

	// generate the room info
	var rinf roomInfo
	//TODO create room

	maxid := big.NewInt(math.MaxUint32)
	var id32 uint32
	roomsLock.Lock()
	for {
		id, err := rand.Int(rand.Reader, maxid)
		if err != nil {
			log.Panicf("Could not generate random id! %v", err)
		}

		id32 = uint32(id.Uint64())

		_, ok := rooms[id32]
		if !ok {
			rooms[id32] = rinf
			break
		}
		// else continue to gen numbers till we find one
	}
	roomsLock.Unlock()
	log.Printf("Creating new room: %x", id32)
}

func serveRoom(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./site/sigl.html")
}

func cleanRooms() {
	for {
		//TODO sleep a smart amount based on previous amounts culled
		time.Sleep(120 * time.Second)

		var found bool
		var id uint32 = 0
		var file string
		now := time.Now()
		for {
			found = false
			roomsLock.RLock()
			for k := range rooms {
				if rooms[k].exp.After(now) {
					id = k
					break
				}
			}
			roomsLock.RUnlock()

			if found {
				roomsLock.Lock()
				// remove the value and delete the file
				file = rooms[id].file
				delete(rooms, id)
				os.Remove(file)
				roomsLock.Unlock()
			} else {
				break
			}
		}
	}
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
	go cleanRooms()

	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
