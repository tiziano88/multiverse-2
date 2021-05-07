self.addEventListener('install', function (event) {
    // Perform install steps
    console.log("SW installed");
});

self.addEventListener('activate', function (event) {
    // Perform install steps
    console.log("SW activated");
});

const handleRequest = async function(event, clients)  {
    const client = await clients.get(event.clientId);
    if (client) {
        var channel = new MessageChannel();
        const p = new Promise((resolve, reject) => {
            channel.port2.onmessage = (m) => {
                console.log("received");
                resolve(m);
            };
            client.postMessage({
                url: event.request.url,
                method: event.request.method,
                s: channel.port1,
            }, [channel.port1]);
        })
        const r = await p;
        return fetch(r.data.url);
    }
}

self.addEventListener('fetch', function (event) {
    event.respondWith(handleRequest(event, self.clients));
});
