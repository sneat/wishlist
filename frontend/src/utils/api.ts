export interface BoardGame {
    id: string;
    name: string;
    thumbnail: string;
    image: string;
    yearPublished: string;
    players: string;
    playingTime?: string;
    rating: number;
    priority: number;
    price: number;
    isOwned?: boolean;
    lastModified: string;
    createdAt: string;
    bggUrl?: string;
    bgoUrl?: string;
}

// Use environment variable or fall back to localhost for development
const API_BASE_URL = import.meta.env.PUBLIC_API_URL || 'http://127.0.0.1:8090';

export async function fetchWishlist(): Promise<BoardGame[]> {
    const response = await fetch(`${API_BASE_URL}/api/v1/bgg-wishlist`);
    const data = await response.json();

    return data.map((item: any) => ({
        id: item.bgg_id,
        name: item.name,
        thumbnail: item.thumbnail,
        image: item.image,
        yearPublished: item.year_published,
        players: item.players,
        playingTime: item.playing_time,
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