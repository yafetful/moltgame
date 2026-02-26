"use client";

import { useTranslations } from "next-intl";
import { useState } from "react";
import Image from "next/image";

const CARDS = [
  {
    key: "texasHoldem" as const,
    icon: "/icons/poker.png",
    count: 12,
    rotate: -6,
    href: "/lobby?game=poker",
    defaultZ: 31,
  },
  {
    key: "werewolf" as const,
    icon: "/icons/werewolves.png",
    count: 8,
    rotate: 6,
    href: "/lobby?game=werewolf",
    defaultZ: 30,
  },
];

export default function GameCards() {
  const t = useTranslations("home");
  const [hovered, setHovered] = useState<number | null>(null);

  return (
    <div className="absolute inset-x-0 bottom-0 z-30 flex items-end justify-center">
      {CARDS.map((card, i) => {
        const isHovered = hovered === i;

        return (
          <button
            key={card.key}
            onMouseEnter={() => setHovered(i)}
            onMouseLeave={() => setHovered(null)}
            className="-mx-5 cursor-pointer transition-all duration-300 ease-out"
            style={{
              transform: `rotate(${isHovered ? 0 : card.rotate}deg)`,
              marginBottom: isHovered ? "16px" : "-32px",
              opacity: isHovered || (hovered === null && i === 0) ? 1 : 0.7,
              zIndex: isHovered ? 40 : card.defaultZ,
            }}
          >
            <div className="flex h-60 w-44 flex-col items-center gap-2 rounded-3xl border-2 border-black bg-white p-5">
              <p className="font-semibold text-base text-black">
                {t(card.key)}
              </p>
              <span className="rounded-full bg-black px-4 py-1 font-semibold text-sm text-white">
                {card.count}
              </span>
              <Image
                src={card.icon}
                alt={t(card.key)}
                width={128}
                height={128}
                className="mt-auto object-contain"
              />
            </div>
          </button>
        );
      })}
    </div>
  );
}
