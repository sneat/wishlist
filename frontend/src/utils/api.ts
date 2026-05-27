export interface BoardGame {
    id: string;
    name: string;
    thumbnail: string;
    image: string;
    yearPublished: number;
    players: string;
    playingTime?: number;
    rating: number;
    priority: number;
    price: number;
    isOwned?: boolean;
    lastModified: string;
    createdAt: string;
    bggUrl?: string;
    bgoUrl?: string;
    description?: string;
    minAge?: number;
    bestPlayerCount?: string;
    bestPlayerCountNumber?: number;
    categories?: string[];
    mechanics?: string[];
    bggRank?: number;
    detailsLastFetched?: string;
}

// Use environment variable or fall back to localhost for development
const API_BASE_URL = import.meta.env.PUBLIC_API_URL || 'http://127.0.0.1:8090';
const BGO_LOCALE = import.meta.env.PUBLIC_BGO_LOCALE || 'en-AU';

export async function fetchWishlist(): Promise<BoardGame[]> {
    const response = await fetch(`${API_BASE_URL}/api/v1/bgg-wishlist`);
    const data = await response.json();

    return data.map((item: any) => ({
        id: item.bgg_id,
        name: item.name,
        thumbnail: item.thumbnail,
        image: item.image,
        yearPublished: item.year_published ?? 0,
        players: item.players,
        playingTime: item.playing_time ?? 0,
        rating: item.rating ?? 0,
        priority: item.priority ?? 0,
        price: item.price ?? 0,
        isOwned: !!item.is_owned,
        lastModified: item.last_modified,
        createdAt: item.created_at,
        bggUrl: `https://boardgamegeek.com/boardgame/${item.bgg_id}`,
        bgoUrl: item.bgo_id ?
            `https://www.boardgameoracle.com/${BGO_LOCALE}/boardgame/price/${item.bgo_id}/` :
            `https://www.boardgameoracle.com/${BGO_LOCALE}/boardgame/search?q=${encodeURI(item.name)}`,
        description: item.description,
        minAge: item.minage,
        bestPlayerCount: item.best_player_count,
        bestPlayerCountNumber: item.best_player_count_number ?? 0,
        categories: item.categories,
        mechanics: item.mechanics,
        bggRank: item.bgg_rank ?? 0,
        detailsLastFetched: item.details_last_fetched,
    }));
}