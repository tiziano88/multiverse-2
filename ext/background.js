var callback = function(details) {
    console.log(details);
    const url = new URL(details.url);
    console.log(url);
    const host = url.host;
    const segments = host.split('.');
    const hash = segments[0];
    console.log(hash);
    const target = "http://localhost:8080/web/" + hash;
    const r = fetch(target);
    return {
        redirectUrl: "http://localhost:8080/web/" + hash,
    };
};
chrome.webRequest.onBeforeRequest.addListener(callback, { urls: ["*://*.meta/*"]}, ["blocking"]);
