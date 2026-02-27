import type { Suit } from "@/components/poker/PokerCard";

const SUIT_MAP: Record<string, Suit> = {
  s: "spades",
  h: "hearts",
  d: "diamonds",
  c: "clubs",
};

const RANK_MAP: Record<string, number> = {
  A: 1,
  "2": 2,
  "3": 3,
  "4": 4,
  "5": 5,
  "6": 6,
  "7": 7,
  "8": 8,
  "9": 9,
  T: 10,
  J: 11,
  Q: 12,
  K: 13,
};

export interface Card {
  suit: Suit;
  value: number;
}

/** Parse a 2-char card string like "Ah" into { suit: "hearts", value: 1 } */
export function parseCard(card: string): Card {
  const rank = card[0];
  const suit = card[1];
  return {
    suit: SUIT_MAP[suit] || "spades",
    value: RANK_MAP[rank] || 1,
  };
}

/** Parse an array of 2-char card strings */
export function parseCards(cards: string[]): Card[] {
  return cards.map(parseCard);
}
