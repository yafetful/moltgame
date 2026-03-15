"use client";

import { useTranslations } from "next-intl";
import { useState, useEffect } from "react";
import { useRouter } from "@/i18n/navigation";
import Image from "next/image";
import { fetchLiveGames, fetchRecentGames } from "@/lib/api";

const CARDS = [
  {
    key: "texasHoldem" as const,
    slug: "poker",
    icon: "/icons/poker.png",
    rotate: -6,
    defaultZ: 31,
    enabled: true,
  },
  {
    key: "werewolf" as const,
    slug: "werewolf",
    icon: "/icons/werewolves.png",
    rotate: 6,
    defaultZ: 30,
    enabled: false,
  },
];

export default function GameCards() {
  const t = useTranslations("home");
  const router = useRouter();
  const [hovered, setHovered] = useState<number | null>(null);
  const [liveByType, setLiveByType] = useState<Record<string, string[]>>({});
  const [recentByType, setRecentByType] = useState<Record<string, string[]>>({});

  useEffect(() => {
    fetchLiveGames().then((games) => {
      // All live games are poker for now
      const ids = games.map((g) => g.game_id);
      setLiveByType({ poker: ids });
    });
    fetchRecentGames().then((games) => {
      const byType: Record<string, string[]> = {};
      for (const g of games) {
        if (!byType[g.game_type]) byType[g.game_type] = [];
        byType[g.game_type].push(g.game_id);
      }
      setRecentByType(byType);
    });
  }, []);

  const handleClick = (slug: string, enabled: boolean) => {
    if (!enabled) return;
    const liveIds = liveByType[slug] || [];
    if (liveIds.length > 0) {
      router.push(`/game/${liveIds[0]}`);
    } else {
      const recentIds = recentByType[slug] || [];
      if (recentIds.length > 0) {
        router.push(`/game/${recentIds[0]}`);
      } else {
        router.push(`/lobby/${slug}`);
      }
    }
  };

  return (
    <div className="absolute inset-x-0 bottom-0 z-30 flex items-end justify-center">
      {CARDS.map((card, i) => {
        const isHovered = hovered === i;
        const liveCount = (liveByType[card.slug] || []).length;
        const recentCount = (recentByType[card.slug] || []).length;
        const isLive = liveCount > 0;
        const displayCount = isLive ? liveCount : recentCount;

        return (
          <button
            key={card.key}
            onMouseEnter={() => setHovered(i)}
            onMouseLeave={() => setHovered(null)}
            onClick={() => handleClick(card.slug, card.enabled)}
            className={`-mx-5 transition-all duration-300 ease-out ${card.enabled ? "cursor-pointer" : "cursor-default"}`}
            style={{
              transform: `rotate(${isHovered ? 0 : card.rotate}deg)`,
              marginBottom: isHovered ? "16px" : "-32px",
              opacity: isHovered || (hovered === null && i === 0) ? 1 : 0.7,
              zIndex: isHovered ? 40 : card.defaultZ,
            }}
          >
            <div className={`flex h-60 w-44 flex-col items-center gap-2 rounded-3xl border-2 border-black bg-white p-5 ${!card.enabled ? "grayscale" : ""}`}>
              <p className="font-semibold text-base text-black">
                {t(card.key)}
              </p>
              <span
                className="rounded-full px-4 py-1 font-semibold text-sm text-white"
                style={{ backgroundColor: isLive ? "#00d74b" : "#000" }}
              >
                {displayCount}
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
