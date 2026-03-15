"use client";

import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import Nav from "@/components/Nav";
import Image from "next/image";
import { useEffect, useState } from "react";
import { fetchLiveGames, fetchRecentGames } from "@/lib/api";

const GAMES = [
  {
    key: "texasHoldem" as const,
    descKey: "texasHoldemDesc" as const,
    slug: "poker",
    icon: "/icons/poker.png",
    scene: "/images/scene-poker.png",
    sceneMobile: "/images/scene-poker-square.png",
    enabled: true,
  },
  {
    key: "werewolf" as const,
    descKey: "werewolfDesc" as const,
    slug: "werewolf",
    icon: "/icons/werewolves.png",
    scene: "/images/scene-werewolf.png",
    sceneMobile: "/images/scene-werewolf-square.png",
    enabled: false,
  },
];

export default function Lobby() {
  const t = useTranslations("lobby");
  const [liveByType, setLiveByType] = useState<Record<string, number>>({});
  const [recentByType, setRecentByType] = useState<Record<string, number>>({});

  useEffect(() => {
    // Live games are all poker for now (no game_type field)
    fetchLiveGames().then((g) => setLiveByType({ poker: g.length }));
    fetchRecentGames().then((games) => {
      const counts: Record<string, number> = {};
      for (const g of games) {
        counts[g.game_type] = (counts[g.game_type] || 0) + 1;
      }
      setRecentByType(counts);
    });
  }, []);

  return (
    <main className="min-h-screen bg-[#fff2eb]">
      <Nav variant="logo" />

      <div className="mx-auto flex max-w-5xl flex-col gap-8 px-8 pt-8 pb-16">
        {GAMES.map((game) => {
          const className = `flex flex-col gap-2 md:gap-4 transition-opacity ${game.enabled ? "hover:opacity-80" : "grayscale opacity-50 cursor-default"}`;
          const inner = (<>
            {/* Header row */}
            <div className="flex items-center justify-between md:px-6">
              {/* Left: icon + name (+ desc on desktop) */}
              <div className="flex items-center gap-1 md:gap-2">
                <Image
                  src={game.icon}
                  alt={t(game.key)}
                  width={64}
                  height={64}
                  className="size-8 md:size-16 object-contain"
                />
                <div className="flex flex-col gap-1">
                  <h2 className="font-semibold text-base md:text-2xl text-black">
                    {t(game.key)}
                  </h2>
                  <p className="hidden md:block font-medium text-xs text-black">
                    {t(game.descKey)}
                  </p>
                </div>
              </div>

              {/* Right: stats */}
              <div className="flex gap-2 md:gap-4 items-center">
                <div className="flex flex-col items-center gap-1">
                  <p className="font-semibold text-xs text-center text-black">
                    {t("liveGames")}
                  </p>
                  <p className="font-black text-base md:text-3xl text-black">
                    {liveByType[game.slug] || 0}
                  </p>
                </div>
                <div className="flex flex-col items-center gap-1">
                  <p className="font-semibold text-xs text-center text-black">
                    {t("gamesPlayed")}
                  </p>
                  <p className="font-black text-base md:text-3xl text-black">
                    {(recentByType[game.slug] || 0).toLocaleString()}
                  </p>
                </div>
              </div>
            </div>

            {/* Mobile: square scene image */}
            <div className="relative aspect-square w-full overflow-hidden rounded-3xl border-4 border-black md:hidden">
              <Image
                src={game.sceneMobile}
                alt={t(game.key)}
                width={2048}
                height={2048}
                className="size-full object-cover"
              />
              {!game.enabled && (
                <div className="absolute inset-0 flex items-center justify-center">
                  <p className="text-4xl font-black text-white drop-shadow-[0_2px_4px_rgba(0,0,0,0.5)]">
                    Coming Soon
                  </p>
                </div>
              )}
            </div>

            {/* Desktop: landscape scene image */}
            <div className="relative hidden overflow-hidden rounded-3xl border-4 border-black md:block">
              <Image
                src={game.scene}
                alt={t(game.key)}
                width={1280}
                height={360}
                className="w-full object-cover"
              />
              {!game.enabled && (
                <div className="absolute inset-0 flex items-center justify-center">
                  <p className="text-4xl font-black text-white drop-shadow-[0_2px_4px_rgba(0,0,0,0.5)]">
                    Coming Soon
                  </p>
                </div>
              )}
            </div>

            {/* Mobile only: description below image */}
            <p className="md:hidden font-medium text-xs text-black">
              {t(game.descKey)}
            </p>
          </>);
          return game.enabled ? (
            <Link key={game.key} href={`/lobby/${game.slug}`} className={className}>
              {inner}
            </Link>
          ) : (
            <div key={game.key} className={className}>
              {inner}
            </div>
          );
        })}
      </div>
    </main>
  );
}
