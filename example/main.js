const e = React.createElement;

function start() {
    ReactDOM.render(
        e(Items, null, null),
        document.getElementById("articles")
    );
}
