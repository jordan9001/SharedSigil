package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log"
	"math"
	"math/big"
	m_rand "math/rand"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

type userConfig struct {
	Clr            string  `json:"clr"`
	Ink            float32 `json:"ink"`
	Depth          float32 `json:"depth"`
	Centered       uint    `json:"centered"`
	Bristles       uint    `json:"bristles"`
	Smoothing      float32 `json:"smoothing"`
	LiftSmoothing  float32 `json:"lift_smoothing"`
	StartSmoothing float32 `json:"start_smoothing"`
}

type dotsConfig struct {
	Clr     string  `json:"clr"`
	Points  uint32  `json:"points"`
	D       float32 `json:"d"`
	Rp      float32 `json:"rp"`
	Pointup bool    `json:"pointup"`
}

type roomConfig struct {
	Bg   string       `json:"bg"`
	Dots []dotsConfig `json:"dots"`
}

type userInfo struct {
	uid       uint32
	submitted bool
	conf      userConfig
}

type roomInfo struct {
	id       uint32
	exp      time.Time
	users    []userInfo
	conf     roomConfig
	filepath string // os version
	flock    *sync.Mutex
}

var roomsLock sync.RWMutex
var rooms map[uint32]roomInfo

var imgPath string

// get_config: give a unique identifier and get back room config
func getConfig(w http.ResponseWriter, r *http.Request) {
	var uc userConfig
	var rc roomConfig
	var submitted bool
	var ok bool = false

	// get id and uid from req
	id_str := r.PostFormValue("id")
	if len(id_str) == 0 {
		// create a random config for a singleplayer game
		ok = true
		uc = randomUserConfig(1)
		rc = randomRoomConfig()
		submitted = false
	} else {

		id64, err := strconv.ParseUint(id_str, 10, 32)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		id := uint32(id64)

		uid_str := r.PostFormValue("uid")
		if len(id_str) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		uid64, err := strconv.ParseUint(uid_str, 10, 32)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		uid := uint32(uid64)

		// lock rooms for reading
		ok = false
		{
			roomsLock.RLock()

			// check the room/user exists
			_, ok = rooms[id]
			if ok {
				ok = false
				for k := range rooms[id].users {
					if uid == rooms[id].users[k].uid {
						ok = true
						rc = rooms[id].conf
						uc = rooms[id].users[k].conf
						submitted = rooms[id].users[k].submitted
						break
					}
				}
			}

			// unlock rooms
			roomsLock.RUnlock()
		}
	}
	// generate a config for the user to return
	// respond
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
	} else {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			Uc        userConfig
			Rc        roomConfig
			Submitted bool
		}{
			Uc:        uc,
			Rc:        rc,
			Submitted: submitted,
		},
		)
	}
}

// send_strokes: sends in completed drawing
func sendStrokes(w http.ResponseWriter, r *http.Request) {
	log.Printf("Got incoming strokes")

	// get id and uid and data from req
	id_str := r.PostFormValue("id")
	if len(id_str) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	id64, err := strconv.ParseUint(id_str, 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	id := uint32(id64)

	uid_str := r.PostFormValue("uid")
	if len(id_str) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	uid64, err := strconv.ParseUint(uid_str, 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	uid := uint32(uid64)

	base64prefix := "data:image/png;base64,"

	data := r.PostFormValue("img")
	if len(data) < len(base64prefix) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(data, base64prefix) {
		log.Printf("Unknown prefix on image!")
		return
	}

	data = data[len(base64prefix):]

	// parse out the file and make sure it matches
	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(data))
	img, _, err := image.Decode(reader)
	if err != nil {
		log.Printf("Failed to properly decode image")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	rgba, ok := img.(*image.NRGBA)
	if !ok {
		log.Printf("Image provided was not nrgba type")
		rct := img.Bounds()
		rgba = image.NewNRGBA(rct)
		draw.Draw(rgba, rct, img, rct.Min, draw.Src)
	}

	// lock the rooms mux for reading
	roomsLock.RLock()
	defer roomsLock.RUnlock()

	_, ok = rooms[id]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ok = false
	usr_idx := 0
	for k := range rooms[id].users {
		if uid == rooms[id].users[k].uid {
			if !rooms[id].users[k].submitted {
				ok = true
				usr_idx = k
			}
			break
		}
	}
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// lock the file mux
	rooms[id].flock.Lock()
	defer rooms[id].flock.Unlock()

	// if there is no file, just create one
	file, err := os.OpenFile(rooms[id].filepath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		log.Printf("Unable to open/create image file")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer file.Close()

	finfo, err := file.Stat()
	if err != nil {
		log.Printf("Unable to stat image file")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// at this point we can count the user as having submitted
	rooms[id].users[usr_idx].submitted = true

	if finfo.Size() <= 0 {
		// new file, just write the stuff in
		err = png.Encode(file, rgba)
		if err != nil {
			log.Printf("Unable to write out fresh image file")
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	// parse the existing image
	e_img, _, err := image.Decode(file)
	if err != nil {
		log.Printf("Unable to parse image already on disk?")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	e_rgba, ok := e_img.(*image.NRGBA)
	if !ok {
		log.Printf("Image on disk was not expected NRGBA?")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	i_bounds := rgba.Bounds()
	e_bounds := e_rgba.Bounds()
	if i_bounds != e_bounds {
		log.Printf("Image sizes don't match?")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// edit the new data
	for y := i_bounds.Min.Y; y < i_bounds.Max.Y; y++ {
		for x := i_bounds.Min.X; x < i_bounds.Max.X; x++ {
			//TODO some better combo than just add or multiply? subtract maybe?

			var r float32
			var g float32
			var b float32
			var a float32

			p := rgba.NRGBAAt(x, y)
			ep := e_rgba.NRGBAAt(x, y)

			// make sure to apply alpha
			ina := float32(p.A) / float32(math.MaxUint8)
			ea := float32(ep.A) / float32(math.MaxUint8)

			r = (float32(p.R) * ina) + (float32(ep.R) * ea)
			if r > math.MaxUint8 {
				r = math.MaxUint8
			}
			g = (float32(p.G) * ina) + (float32(ep.G) * ea)
			if g > math.MaxUint8 {
				g = math.MaxUint8
			}
			b = (float32(p.B) * ina) + (float32(ep.B) * ea)
			if b > math.MaxUint8 {
				b = math.MaxUint8
			}
			a = float32(p.A) + float32(ep.A)
			if a > math.MaxUint8 {
				a = math.MaxUint8
			}

			e_rgba.SetNRGBA(x, y, color.NRGBA{uint8(r), uint8(g), uint8(b), uint8(a)})
		}
	}

	//TODO
	// write the new data into a new file
	// replace the old file (so requests wont get half written files)

	file.Seek(0, io.SeekStart)
	err = png.Encode(file, e_rgba)
	if err != nil {
		log.Printf("Unable to write out changed image")
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// get_done: get back x/total submitted for your room, polled
func getDone(w http.ResponseWriter, r *http.Request) {
	var done int = 0
	var outof int = 0
	var submitted int = 0

	// get id and uid from req

	id_str := r.PostFormValue("id")
	if len(id_str) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	id64, err := strconv.ParseUint(id_str, 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	id := uint32(id64)

	uid_str := r.PostFormValue("uid")
	if len(id_str) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	uid64, err := strconv.ParseUint(uid_str, 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	uid := uint32(uid64)

	var ok bool = false
	{
		roomsLock.RLock()

		_, ok = rooms[id]

		if ok {
			ok = false
			for k := range rooms[id].users {
				if rooms[id].users[k].submitted {
					done += 1
				}
				if uid == rooms[id].users[k].uid {
					if rooms[id].users[k].submitted {
						submitted = 1
					}
					ok = true
				}
			}
		}

		outof = len(rooms[id].users)

		roomsLock.RUnlock()
	}

	// respond based on ok and done, outof
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
	} else {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]int{done, outof, submitted})
	}
}

func randomColorStr(lumabase, lumad, satmax float32) string {
	// js makes this easy because we can do hsl colors
	luma := (m_rand.Float32() * 2.0 * lumad) + (lumabase - lumad)

	sat := m_rand.Float32() * satmax

	hue := m_rand.Int() % 360

	luma_i := int(luma)
	sat_i := int(sat)

	return fmt.Sprintf("hsl(%d, %d%%, %d%%)", hue, sat_i, luma_i)
}

func randomRoomConfig() roomConfig {
	var conf roomConfig

	//conf.Bg = "#3f3f4d"
	conf.Bg = randomColorStr(60.0, 12.0, 12.0)

	num := m_rand.Int() % 5
	conf.Dots = make([]dotsConfig, num)
	for i := 0; i < num; i++ {
		var pup bool = (m_rand.Int() & 1) == 1
		var pts uint32 = (m_rand.Uint32() & 0x7) + 3
		var dia float32 = 2.0 / 3.0
		dia += m_rand.Float32() - 0.5
		conf.Dots[i] = dotsConfig{
			Clr:     "#000000",
			Points:  pts,
			D:       dia,
			Rp:      3,
			Pointup: pup,
		}
	}

	return conf
}

func randomUserConfig(num uint64) userConfig {
	return userConfig{
		Clr:            randomColorStr(24.0, 15.0, 45.0),
		Ink:            240000.0 / float32(num),
		Depth:          72.0,
		Centered:       uint((m_rand.Uint32() % 12) + 6),
		Bristles:       uint((m_rand.Uint32() % 90) + 60),
		Smoothing:      0.21,
		LiftSmoothing:  0.06,
		StartSmoothing: 0.021,
	}
}

// create_room: create a room for x people and returns links (used in beginning)
func createRoom(w http.ResponseWriter, r *http.Request) {
	// get how many players
	num_str := r.PostFormValue("num")
	if len(num_str) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	num, err := strconv.ParseUint(num_str, 10, 8)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// generate the room info
	var rinf roomInfo

	// create room
	rinf.exp = time.Now().Add(time.Hour * 21)
	rinf.users = make([]userInfo, num)
	rinf.flock = &sync.Mutex{}
	rinf.conf = randomRoomConfig()

	var resp []uint32 = make([]uint32, num+1)
	for i := 0; i < int(num); i++ {
		// loop to generate uids and make sure we don't reuse one
		for {
			uid := m_rand.Uint32()
			ok := true
			for j := 0; j < i; j++ {
				if rinf.users[j].uid == uid {
					ok = false
					break
				}
			}
			if ok {
				rinf.users[i].uid = uid
				resp[i] = uid
				break
			}
		}

		rinf.users[i].conf = randomUserConfig(num)
	}

	maxid := big.NewInt(math.MaxUint32)
	var id32 uint32
	var fname string
	{
		roomsLock.Lock()
		for {
			id, err := rand.Int(rand.Reader, maxid)
			if err != nil {
				log.Panicf("Could not generate random id! %v", err)
			}

			id32 = uint32(id.Uint64())

			_, ok := rooms[id32]
			if !ok {
				rinf.id = id32
				fname = strconv.FormatUint(uint64(id32), 10) + ".png"
				rinf.filepath = path.Join(imgPath, fname)
				rooms[id32] = rinf
				break
			}
			// else continue to gen numbers till we find one
		}
		roomsLock.Unlock()
	}

	log.Printf("Creating new room: %x (%q)", id32, rinf.filepath)
	// file not actually created until there is something to put

	// add in the room id
	resp[num] = id32

	// Generate the response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func serveRoom(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./site/sigl.html")
}

func cleanRooms() {
	for {
		log.Printf("Clean Sweep")
		//TODO sleep a smart amount based on previous amounts culled
		time.Sleep(1 * time.Hour)

		var found bool
		var id uint32 = 0
		var file string
		now := time.Now()
		for {
			found = false
			{
				roomsLock.RLock()
				for k := range rooms {
					if rooms[k].exp.After(now) {
						id = k
						found = true
						break
					}
				}
				roomsLock.RUnlock()
			}

			if found {
				log.Printf("Found expired room: %v", id)
				roomsLock.Lock()
				// remove the value and delete the file
				file = rooms[id].filepath
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
	var imgdir = flag.String("dir", "testimgs", "Path to image directory")

	flag.Parse()

	sd, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		log.Panicf("Error getting a seed: %v", err)
	}
	m_rand.Seed(sd.Int64())
	rooms = make(map[uint32]roomInfo)

	log.Printf("Starting up sigl server on port %v @ %v", *port, *imgdir)

	imgPath = *imgdir

	fileServer := http.FileServer(http.Dir("site"))
	http.Handle("/", fileServer)
	http.HandleFunc("/s/", serveRoom)
	sigilServer := http.FileServer(http.Dir(imgPath))
	http.Handle("/sigils/", http.StripPrefix("/sigils/", sigilServer))
	http.HandleFunc("/api/get_config", getConfig)
	http.HandleFunc("/api/send_strokes", sendStrokes)
	http.HandleFunc("/api/get_done", getDone)
	http.HandleFunc("/api/create_room", createRoom)

	// start goroutine to clean up timed-out rooms
	go cleanRooms()

	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
