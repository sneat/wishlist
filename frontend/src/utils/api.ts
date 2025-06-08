export interface BoardGame {
    id: string;
    name: string;
    thumbnail: string;
    image: string;
    yearPublished: string;
    players: string;
    rating: number;
    priority: number;
    price: number;
    isOwned?: boolean;
    lastModified: string;
    createdAt: string;
    bggUrl?: string;
    bgoUrl?: string;
}

export async function fetchWishlist(): Promise<BoardGame[]> {
    const response = await fetch(`https://wishlist.mcmillan.id.au/api/v1/bgg-wishlist`);
    const data = await response.json();

    return data.map((item: any) => ({
        id: item.bgg_id,
        name: item.name,
        thumbnail: item.thumbnail,
        image: item.image,
        yearPublished: item.year_published,
        players: item.players,
        rating: item.rating,
        priority: parseInt(item.priority, 10) || 0,
        price: parseInt(item.price, 10) || 0,
        isOwned: !!item.is_owned,
        lastModified: item.last_modified,
        createdAt: item.created_at,
        bggUrl: `https://boardgamegeek.com/boardgame/${item.bgg_id}`,
        bgoUrl: item.bgo_id ?
            `https://www.boardgameoracle.com/en-AU/boardgame/price/${item.bgo_id}/` :
            `https://www.boardgameoracle.com/en-AU/boardgame/search?q=${encodeURI(item.name)}`,
    }));
}