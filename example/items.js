class Items extends React.Component {
    constructor() {
        super();
        this.state = {
            items: [
                {id: 0, by: "user", score: 126, title: "Pokegb: A gameboy emulator that only plays Pokémon Blue, in 68 lines of C++", url: "https://binji.github.io/posts/pokegb/"},
                {id: 1, by: "user", score: 126, title: "Pokegb: A gameboy emulator that only plays Pokémon Blue, in 68 lines of C++", url: "https://binji.github.io/posts/pokegb/"},
                {id: 2, by: "user", score: 126, title: "Pokegb: A gameboy emulator that only plays Pokémon Blue, in 68 lines of C++", url: "https://binji.github.io/posts/pokegb/"},
                {id: 3, by: "user", score: 126, title: "Pokegb: A gameboy emulator that only plays Pokémon Blue, in 68 lines of C++", url: "https://binji.github.io/posts/pokegb/"},
                {id: 4, by: "user", score: 126, title: "Pokegb: A gameboy emulator that only plays Pokémon Blue, in 68 lines of C++", url: "https://binji.github.io/posts/pokegb/"},
            ]
        };
    }

    render( ){
        return e('div', null,
            this.state.items.map((item) => e(Item, {item:item, key: item.id}, null)));
    }

    componentDidMount() {
        this.go();
    }

    async go() {
        const topStories = await (await fetch("https://hacker-news.firebaseio.com/v0/topstories.json")).json()
        console.log(topStories);
        var items = [];
        for (var i = 0; i < env.numberOfArticles; i++) {
            const itemId = topStories[i];
            const item = await(await fetch("https://hacker-news.firebaseio.com/v0/item/"+itemId+".json")).json();
            items.push(item);
        }
        console.log(items);
        this.setState({
            items: items
        })
    }
}
