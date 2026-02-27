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
  },
  {
    key: "werewolf" as const,
    descKey: "werewolfDesc" as const,
    slug: "werewolf",
    icon: "/icons/werewolves.png",
    scene: "/images/scene-werewolf.png",
  },
];

export default function Lobby() {
  const t = useTranslations("lobby");
  const [liveCount, setLiveCount] = useState(0);
  const [recentCount, setRecentCount] = useState(0);

  useEffect(() => {
    fetchLiveGames().then((g) => setLiveCount(g.length));
    fetchRecentGames().then((g) => setRecentCount(g.length));
  }, []);

  return (
    <main className="min-h-screen bg-[#fff2eb]">
      <Nav variant="logo" />

      <div className="mx-auto flex max-w-5xl flex-col gap-8 px-8 pt-8 pb-16">
        {GAMES.map((game) => (
          <Link
            key={game.key}
            href={`/lobby/${game.slug}`}
            className="flex flex-col gap-4 transition-opacity hover:opacity-80"
          >
            {/* Header row */}
            <div className="flex items-center justify-between px-6">
              {/* Left: icon + text */}
              <div className="flex items-center gap-2">
                <Image
                  src={game.icon}
                  alt={t(game.key)}
                  width={64}
                  height={64}
                  className="size-16 object-contain"
                />
                <div className="flex flex-col gap-1">
                  <h2 className="font-semibold text-2xl text-black">
                    {t(game.key)}
                  </h2>
                  <p className="font-medium text-xs text-black">
                    {t(game.descKey)}
                  </p>
                </div>
              </div>

              {/* Right: stats */}
              <div className="flex gap-4 items-center">
                <div className="flex flex-col items-center gap-1">
                  <p className="font-semibold text-base text-center text-black">
                    {t("liveGames")}
                  </p>
                  <p className="font-black text-3xl text-black">
                    {liveCount}
                  </p>
                </div>
                <div className="flex flex-col items-center gap-1">
                  <p className="font-semibold text-base text-center text-black">
                    {t("gamesPlayed")}
                  </p>
                  <p className="font-black text-3xl text-black">
                    {recentCount.toLocaleString()}
                  </p>
                </div>
              </div>
            </div>

            {/* Scene image */}
            <div className="overflow-hidden rounded-3xl border-4 border-black">
              <Image
                src={game.scene}
                alt={t(game.key)}
                width={1280}
                height={360}
                className="w-full object-cover"
              />
            </div>
          </Link>
        ))}
      </div>
    </main>
  );
}
