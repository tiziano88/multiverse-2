class Item extends React.Component {
    constructor(props) {
        super(props);
        this.state = {
            item: props.item,
        };
    }

    render( ){
        return e('div', { className: 'border py-3 my-3 bg-' + env.colour + '-200 hover:bg-' + env.colour + '-300'},
            e('span', { className: 'px-5 w-24 inline-block'}, '[' + this.state.item.score + ']'),
            e('a', {href: this.state.item.url, className: 'underline'}, this.state.item.title),
            e('span', {className: "p-2"}, 'by'),
            e('span', null, this.state.item.by),
        );
    }
}
