'use strict';


let start_btn = document.getElementById("startbtn");
let num = document.getElementById("num");
let links = document.getElementById("links");

start_btn.addEventListener("click", function(evt) {
    let v = parseInt(num.value);
    if (v > 0) {
        if (v == 1) {
            // go to the single player page
            window.location.href += 'sigl.html'
            return;
        }

        // disable the button, so it isn't pressed a ton
        start_btn.disabled = true;
        start_btn.value = "•••";

        // create a new room, and display the links to distribute
        let formdata = new FormData();
        formdata.append('num', v);

        fetch("/api/create_room", {
            method: 'POST',
            body: formdata,
            redirect: 'manual',
        }).then((resp) => {
            //TODO handle a bad response
            return resp.json();
        }).then((data) => {
            if (data.length == (v+1)) {
                let id = data[v];
                for (let i=0; i < v; i++) {
                    let link = window.location.href + 's/' + id + '.' + data[i]
                    let li = document.createElement('li');
                    let a = document.createElement('a');
                    li.appendChild(a);
                    a.href = link;
                    let atxt = document.createTextNode(link);
                    a.appendChild(atxt);
                    links.appendChild(li);
                }
            }
            console.log(data);
        });
    }
}, false);