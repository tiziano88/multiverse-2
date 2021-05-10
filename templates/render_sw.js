self.addEventListener('install', function (event) {
    // Perform install steps
    console.log("SW1 installed");
    self.skipWaiting();
});

self.addEventListener('activate', function (event) {
    // Perform install steps
    console.log("SW1 activated");
    self.skipWaiting();
});

var overrides = {};

const handleRequest = async function(event, clients)  {
    console.log("event2: ", event);
    const client = await clients.get(event.clientId);
    if (client) {
        var channel = new MessageChannel();
        const p = new Promise((resolve, reject) => {
            channel.port2.onmessage = (m) => {
                resolve(m);
            };
            client.postMessage({
                url: event.request.url,
                method: event.request.method,
                s: channel.port1,
            }, [channel.port1]);
        })
        const r = await p;
        if (r.data.upload) {
            console.log('caching ' + r.data.url);
            const response = await fetch(r.data.url, {redirect: "follow"});
            // console.log("download response: ", response);
            const data = new FormData();
            data.append("file", await response.blob(), "file");
            const uploadResponse = await fetch("/upload", { method: "POST", body: data });
            // console.log("upload response: ", uploadResponse);
            const hash = await uploadResponse.text();
            overrides[r.data.url] = hash;
            console.log(JSON.stringify(overrides, {}, 2));
            return fetch("/h/" + hash);
        }
        return fetch(r.data.url);
    } else {
        return fetch(event.request);
        // return new Response(null, {status: 505});
    }
    /*
    const request = event.request;
    const urlToHash = {
        "http://upload.wikimedia.org/wikipedia/commons/thumb/5/53/Tizian_090.jpg/440px-Tizian_090.jpg": "bafkreiag6clwepuduyo6nhyyqcj6rbpnnj6e3i7xc7uezxq7nc7nban44e"
    };
    const url = request.url;
    if (url.startsWith("http://localhost:8080/")
        || url.startsWith("https://multiverse-312721.nw.r.appspot.com/")
        ) {
        // Default.
        return fetch(request);
    } else if (request.method != "GET") {
        return new Response(null, {status: 500});
    } else if (url.startsWith("https://mv/")) {
        const hash = url.replace("https://mv/", "");
        console.log("hash: ", hash);
        return fetch("/h/" + hash);
    } else {
        const hash = urlToHash[url];
        if (hash != undefined) {
            console.log("url mapped to hash: ", hash);
            return fetch("/h/" + hash);
        } else {
            return new Response(null, {status: 500});
        }
    }
    */
}

self.addEventListener('fetch', function (event) {
    console.log("fetch event", event.request);
    if (event.request.headers["multiverse-fetch"]) {
        console.log("skip");
        // default
    } else {
        event.respondWith(handleRequest(event, self.clients));
    }
});
