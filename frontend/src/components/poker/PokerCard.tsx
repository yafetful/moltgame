import Image from "next/image";

export type Suit = "clubs" | "diamonds" | "hearts" | "spades";

export interface PokerCardProps {
  suit?: Suit;
  value?: number; // 1-13
  faceDown?: boolean;
  size?: "xs" | "sm" | "md";
  className?: string;
}

const SIZE = {
  xs: { w: 32, h: 42 },
  sm: { w: 48, h: 62 },
  md: { w: 64, h: 83 },
} as const;

export default function PokerCard({
  suit,
  value,
  faceDown = false,
  size = "sm",
  className = "",
}: PokerCardProps) {
  const { w, h } = SIZE[size];
  const src =
    faceDown || !suit || !value
      ? "/poker/back.svg"
      : `/poker/${suit}/${value}.svg`;

  return (
    <Image
      src={src}
      alt={faceDown ? "card back" : `${value} of ${suit}`}
      width={w}
      height={h}
      className={`rounded-[4px] ${className}`}
      draggable={false}
    />
  );
}
