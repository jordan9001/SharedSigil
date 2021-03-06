'use strict';

function randbm() {
    var u = 0;
    var v = 0;
    while (u === 0) {
        u = Math.random();
    }
    while (v === 0) {
        v = Math.random();
    }

    // transform to normal dist
    return Math.sqrt(-2.0 * Math.log(u)) * Math.cos(2.0 * Math.PI * v);
}

function randclt(n) {
    let v = 0.0
    for (let i = 0; i < n; i++) {
        v += Math.random();
    }
    v /= n;
    v *= 2.0;
    v -= 1.0;
    return v;
}

let SigilBrush = class {
    // fields
    ctx = null;
    pts = []; // [[x,y, weight],]
    depth = 0.0;
    prev_pos = [0.0, 0.0];
    prev_vel = [0.0, 0.0];
    isdown = false;
    a = 1.0;
    ia = 0.0;
    startmul = 0.0;
    lifta = 1.0;
    liftia = 0.0;
    mula = 1.0;
    mulia = 0.0;
    ink = 0.0;
    clr = "#000000";

    constructor(ctx, c) {
        this.ctx = ctx;
        this.depth = c.depth;
        this.a = c.smoothing;
        this.ia = (1 - c.smoothing);
        this.lifta = c.lift_smoothing;
        this.liftia = (1 - c.lift_smoothing);
        this.mula = c.start_smoothing;
        this.mulia = (1 - c.start_smoothing);
        this.ink = c.ink;
        this.clr = c.clr;


        // a brush is a collection points with sizes
        // generate the points and weights
        // gausian curve fall off?
        
        // pressure on the points will add to the points, so weight can be negative?

        let hd = (this.depth / 2.0);
        for (let i = 0; i < c.bristles; i++) {
            let central = randclt(c.centered);
            let ang = Math.random() * Math.PI;
            let x = Math.cos(ang) * central * hd;
            let y = Math.sin(ang) * central * hd;
            let w = ((1.0 - Math.abs(central)) - 0.5) * 2.0;
            w *= (Math.random() * 0.1) + 0.95;
            this.pts.push([x, y, w])
        }

        // Things that could be cool: inertia, splay, etc
    }

    down() {
        this.isdown = true;
        // set stroke start at zero, get us a smooth fade in
        this.startmul = 1.0;
    }

    up() {
        this.isdown = false;
    }

    update_pos(x,y) {
        // draw to the new averaged position
        let prev_x = this.prev_pos[0];
        let prev_y = this.prev_pos[1];
        let new_x = (prev_x * this.ia) + (x * this.a);
        let new_y = (prev_y * this.ia) + (y * this.a);

        // update lift based on velocity
        // (fake vel with no dt)
        let vel_x = new_x - prev_x;
        let vel_y = new_y - prev_y;
        vel_x = (this.prev_vel[0] * this.liftia) + (vel_x * this.lifta);
        vel_y = (this.prev_vel[1] * this.liftia) + (vel_y * this.lifta);

        // update prev
        this.prev_vel[0] = vel_x;
        this.prev_vel[1] = vel_y;
        this.prev_pos[0] = new_x;
        this.prev_pos[1] = new_y;

        // for each point in pts draw offset line from prev to new
        if (this.isdown && this.ink > 0.0) {
            // break out early if we are out of bounds
            for (let px of [new_x, prev_x]) {
                if (px < 0 || px > this.ctx.canvas.width) {
                    return false;
                }
            }
            for (let py of [new_y, prev_y]) {
                if (py < 0 || py > this.ctx.canvas.height) {
                    return false;
                }
            }


            let spd2 = (vel_x * vel_x) + (vel_y * vel_y);
            let spd = Math.sqrt(spd2);
            spd = Math.sqrt(spd);
            let maxspd = 2.7;
            let minspd = 1.0;
            // map max/min to spd from -1 to 1
            let targlift_ = (((spd - minspd) / (maxspd - minspd)) * -2.0) + 1.0;
            let targlift = targlift_;
            //targlift = Math.max(-0.75, targlift);
            targlift = Math.min(1.0, targlift);

            //console.log(spd, targlift);

            this.startmul = (this.startmul * this.mulia);
            let fademul = this.startmul * 2;
            // fade as ink runs out thin out
            fademul += (1.5 - Math.min(1.5, this.ink / 3000.0));

            let sz_fac = targlift - fademul;

            this.ctx.lineCap = "round";
            this.ctx.strokeStyle = this.clr;

            let inkamt = 0.0;
            let dx = new_x - prev_x;
            let dy = new_y - prev_y;
            let ln = Math.sqrt((dx * dx) + (dy*dy));

            for (let pt of this.pts) {
                if ((pt[2] + sz_fac) <= 0.0) {
                    continue;
                }

                this.ctx.beginPath();
                let lw = pt[2] + sz_fac;
                this.ctx.lineWidth = lw;
                this.ctx.moveTo(pt[0] + prev_x, pt[1] + prev_y);
                this.ctx.lineTo(pt[0] + new_x, pt[1] + new_y);
                this.ctx.stroke();

                inkamt += ln * lw;
            }
            this.ink -= inkamt;

            return true;
        }
        return false;
    }
}

let SigilCanvas = class {
    // fields
    ctx = null;
    w = 0;
    h = 0;
    enabled = false;
    ink = 0.0;
    brush = null;
    bg = null;
    inkpot = null;
    inkmax = 0.0;
    done_cb = null;

    constructor(element, bg, pot, done_cb, start_cb) {
        this.ctx = element.getContext("2d");
        // initial state
        this.w = element.clientWidth;
        this.h = element.clientHeight;
        element.width = this.w;
        element.height = this.h;
        this.bg = bg;
        this.inkpot = pot;
        this.done_cb = done_cb;
        this.firstdraw = true;

        this.brushdata = this.ctx.createImageData(this.w, this.h);
        
        // set up listeners
        let that = this;

        let moveevt = function(evt) {
            if (!that.enabled) {
                return;
            }
            let cx = evt.clientX;
            let cy = evt.clientY;
            if (cx === undefined || cy === undefined) {
                // try touch event
                if (evt.touches === undefined || evt.touches.length < 1) {
                    console.log("Unknown move event");
                    return;
                }
                cx = evt.touches[0].clientX;
                cy = evt.touches[0].clientY;
            }

            let crect = element.getBoundingClientRect();
            let x = cx - crect.x;
            let y = cy - crect.y;

            // let draw
            // no clearing so we can be nice and smooth
            // we put our background on the div behind us
            let drew = that.brush.update_pos(x, y);
            if (drew) {
                if (that.firstdraw) {
                    that.firstdraw = false;
                    start_cb();
                }
                // update inkpot
                let potw = (that.brush.ink / that.inkmax) * that.w;
                if (potw < 0.0) {
                    potw = 0;
                }
                that.inkpot.style.width = potw + 'px';

                // call callback if we ran out of ink
                if (that.brush.ink <= 0.0) {
                    that.enable(false);
                    that.writeOut();
                }
            }
        };
        document.addEventListener("mousemove", moveevt, false);
        document.addEventListener("touchmove", moveevt, false);

        let downevt = function(evt) {
            if (!that.enabled) {
                return;
            }

            that.brush.down();
        };
        document.addEventListener("mousedown", downevt, false);
        document.addEventListener("touchstart", downevt, false);

        let upevt = function(evt) {
            if (!that.enabled) {
                return;
            }

            that.brush.up();
        };
        document.addEventListener("mouseup", upevt, false);
        document.addEventListener("touchend", upevt, false);
    }

    setBoard(c) {
        // set board settings based on config
        // create brush
        this.brush = new SigilBrush(this.ctx, c.brush);

        this.bg.style.backgroundColor = c.bg;

        // generate a point pattern to draw on
        this.genGuides(c.dots)

        // setup inkpot
        this.inkmax = c.brush.ink;
        this.inkpot.style.height = 15 + "px";
        this.inkpot.style.backgroundColor = c.brush.clr;
        this.inkpot.style.width = this.w + "px";
    }

    genGuides(c) {
        // create an image on the canvas, then set the bg from the dataURL
        // this way we don't have to clear the canvas while drawing, and can still extract the ink after

        let xoff = this.w / 2.0;
        let yoff = this.h / 2.0;

        for (let conf of c) {
            this.ctx.fillStyle = conf.clr;

            let angoff = -Math.PI / 2.0;
            let angdiv = (Math.PI*2) / conf.points;

            let rad = (this.w * conf.d) / 2.0;

            if (!conf.pointup) {
                angoff += angdiv / 2.0; // make top flat
            }
            for (let i = 0; i < conf.points; i++) {
                this.ctx.beginPath();

                let x = Math.cos((angdiv * i) + angoff) * rad;
                let y = Math.sin((angdiv * i) + angoff) * rad;

                this.ctx.arc(x + xoff, y + yoff, conf.rp, 0, 2*Math.PI);
                this.ctx.fill();
            }

        }

        var img = canvas.toDataURL("image/png");
        this.bg.style.backgroundImage = "url(" + img + ")";
        this.ctx.clearRect(0, 0, this.w, this.h);
    }

    writeOut() {
        this.enable(false);

        if (this.done_cb) {
            this.done_cb();
        }
    }

    enable(b) {
        this.enabled = b;
        if (!this.enabled) {
            this.brush.up();
        }
    }
}

// Server class that will reach out to the server for settings
let SigilServer = class {
    state = "UNCONNECTED";
    gettingResult = false;
    btn = null;
    btn_val = "";
    canvas = null;
    subcount = 0;
    subtotal = 0;
    id = 0;
    uid = 0;

    constructor(btn, canvas) {
        // server has endpoints at /api/ for:
        // get_config: give a unique identifier and get back room config
        // send_strokes: sends in completed drawing
        // get_done: get back x/total submitted for your room, poll this
        // create_room: create a room for x people and returns links (used in beginning)

        // unique identifier is the last part of the url
        let l = window.location.pathname.split('/');
        let ids = l[l.length - 1].split('.');
        this.id = ids[0];
        this.uid = ids[1];
        console.log("Got ID: " + this.id + " UID: " + this.uid);

        this.btn = btn;
        this.btn_val = "SEND";
        this.canvas = canvas;
        btn.disabled = true;
        let that = this;
        btn.addEventListener("click", function() {
            if (that.btn_val == 'SEND') {
                that.sendPaint();
            } else if (that.btn_val == 'SAVE') {
                DownloadImg(that.canvas);
            }
        }, false);

        // start timed callback to poll for how many people are done
        setTimeout(function() {that.pollDone();}, 12000);
    }

    getConf(cb) {
        this.state = "GETTING CONF"

        let that = this;

        let formdata = new FormData();
        formdata.append("id", this.id);
        formdata.append("uid", this.uid);

        fetch("/api/get_config", {
            method: 'POST',
            body: formdata,
        }).then((resp) => {
            return resp.json();
        }).then((data) => {
            // sanity check top level of data
            if (!data.hasOwnProperty("Uc") || !data.hasOwnProperty("Rc") || !data.hasOwnProperty("Submitted")) {
                console.log("Unexpected response to get_config");
                that.state = "ERROR";
                return;
            }

            // send config along
            cb(data);
        });
    }

    sendPaint() {
        this.state = 'SENDING';

        // send the drawing to the server, then get the image and see how many are done

        console.log("Sending strokes");

        let img = this.canvas.toDataURL("image/png");
        if (!img.startsWith("data:image/png;base64,")) {
            console.log("Unknown DataURL type!");
        }

        let formdata = new FormData();
        formdata.append("img", img);
        formdata.append("id", this.id);
        formdata.append("uid", this.uid);

        let that = this;

        fetch("/api/send_strokes", {
            method: 'POST',
            body: formdata,
        }).then((resp) => {
            //TODO check we got a good response
            that.setUpdating();
        })
    }

    startDrawing() {
        this.state = 'DRAWING';
        this.btn.disabled = false;
    }

    getUpdate() {
        console.log("Getting result");

        let that = this;

        let path = "/sigils/" + this.id + ".png"
        fetch(path).then((resp) => {
            return resp.blob();
        }).then((data) => {
            let ctx = that.canvas.getContext('2d');
            let img = new Image();
            img.onload = function() {
                ctx.clearRect(0, 0, that.canvas.clientWidth, that.canvas.clientHeight);
                ctx.drawImage(img, 0, 0);
            };

            img.src = URL.createObjectURL(data);
        });
    }

    pollDone() {
        if (this.subtotal != 0 && this.subcount == this.subtotal) {
            // stop polling
            return;
        }
        // call the get_done endpoint
        let that = this;

        let formdata = new FormData();
        formdata.append("id", this.id);
        formdata.append("uid", this.uid);

        fetch("/api/get_done", {
            method: 'POST',
            body: formdata,
        }).then((resp) => {
            return resp.json();
        }).then((data) => {
            let done = data[0];
            let total = data[1];
            if (done > that.subcount) {
                that.subcount = done;
                that.subtotal = total;
                that.btn.value = that.btn_val;
                that.btn.value += "("+that.subcount+"/"+that.subtotal+")";

                if (that.gettingResult) {
                    that.getUpdate();
                }
            }
        });

        // loop until we see that everyone is done
        setTimeout(function() {that.pollDone();}, 9000);
    }

    setUpdating() {
        // go get the image from the server and set so that we keep doing so when the poll gets an updated count
        this.state = "DONE";
        this.btn_val = "SAVE";
        this.btn.value = this.btn_val;
        if (this.subcount != 0) {
            this.btn.value += "("+this.subcount+"/"+this.subtotal+")";
        }

        this.getUpdate();
        this.gettingResult = true;
    }
}

function Setup() {
    // if we are not at a /s/# uri, then just go single right away
    if (!window.location.pathname.startsWith("/s/")) {
        SingleSetup();
        return;
    }

    let canvas = document.getElementById("canvas");

    let btn = document.getElementById("btn");
    let serv = new SigilServer(btn, canvas);
    serv.getConf(function(conf) {
        if (conf == undefined) {
            console.log("Error, could not get conf");
            SingleSetup();
            return;
        }

        let platform = document.getElementById("platform");
        let inkpot = document.getElementById("inkpot");
        let sc = new SigilCanvas(canvas, platform, inkpot, function() {
            serv.sendPaint();
        }, function() {
            serv.startDrawing()
        });

        // convert conf to usable config
        let config = conf.Rc;
        config.brush = conf.Uc;
        sc.setBoard(config);

        if (!conf.Submitted) {
            sc.enable(true);
        } else {
            // if we have already submitted then just have the server start updating the canvas as appropriate
            serv.setUpdating();
        }
    });
}

function DownloadImg(canvas) {
    // get image data
    console.log("Saving");
    var img = canvas.toDataURL("image/png");
    img = img.replace("image/png", "image/octet-stream");

    var a = document.createElement('a');
    a.href = img;
    a.download = "brushstrokes.png";
    document.body.appendChild(a);
    a.click();
}

function SingleSetup() {
    let canvas = document.getElementById("canvas");
    let platform = document.getElementById("platform");
    let inkpot = document.getElementById("inkpot");
    let btn = document.getElementById("btn");
    btn.value = "SAVE";
    btn.disabled = true;
    btn.addEventListener("click", function() {
        console.log("Save button clicked");
        btn.disabled = true;
        DownloadImg(canvas);
        btn.disabled = false;
    }, false);
    let sc = new SigilCanvas(canvas, platform, inkpot, function(){}, function(){
        btn.disabled = false;
    });

    // still get config from server
    fetch("/api/get_config", {
        method: 'GET',
    }).then((resp) => {
        return resp.json();
    }).then((data) => {
        // sanity check top level of data
        if (!data.hasOwnProperty("Uc") || !data.hasOwnProperty("Rc")) {
            console.log("Unexpected response to get_config");
            return;
        }

        let config = data.Rc;
        config.brush = data.Uc;
        sc.setBoard(config);

        // send config along
        sc.setBoard(config);
        sc.enable(true);
    });

    /*let config = {
        brush: {
            depth: 72,                  // size of brush
            centered: 9,                // how centerd the normal distrobution
                                        //       1 is flat, large more pointed
            bristles: 96,               // number of bristles
            smoothing: 0.21,            // how much to smooth mouse path
            lift_smoothing: 0.06,       // how much to smooth mouse velocity
            start_smoothing: 0.021,     // how quick to ease into start of stroke
            ink: 180000,                // amount of ink for brush
            clr: "#000000",             // color of ink
        },
        bg: "#3f3f4d",                  // background behind canvas
        dots: [
            {
                clr: "#000000",         // color of guide dots
                points: 5,              // number of guides
                d: 2.0/3.0,             // diameter of guide circle as ratio of canvas height
                rp: 1.5,                  // dot radius in pixels
                pointup: true,          // point at top, or flat
            },
        ],
    };*/
}


Setup();