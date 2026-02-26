"use client";

import { useTranslations } from "next-intl";
import { useParams } from "next/navigation";
import { Link } from "@/i18n/navigation";
import { useState } from "react";
import Image from "next/image";
import Nav from "@/components/Nav";

const GAME_CONFIG = {
  poker: {
    titleKey: "texasHoldem" as const,
    icon: "/icons/poker.png",
    // Poker: SVG is the complete card shape (oval table with border)
    type: "svg-shape" as const,
    tableImage: "/images/table-poker.svg",
    aspect: "aspect-[200/96]",
  },
  werewolf: {
    titleKey: "werewolf" as const,
    icon: "/icons/werewolves.png",
    // Werewolf: CSS rounded rect with decorative SVG overlay
    type: "decorated" as const,
    tableImage: "/images/table-werewolf-decor.svg",
    aspect: "aspect-[200/128]",
  },
} as const;

// Mock table data
const MOCK_TABLES = Array.from({ length: 12 }, (_, i) => ({
  id: String(12345678 + i),
  time: "14:58",
}));

type Tab = "live" | "replay";

export default function GameLobby() {
  const t = useTranslations("lobby");
  const params = useParams();
  const gameSlug = params.game as keyof typeof GAME_CONFIG;
  const [tab, setTab] = useState<Tab>("live");

  const config = GAME_CONFIG[gameSlug];
  if (!config) return null;

  const liveCount = 123;
  const replayCount = 1847;

  return (
    <main className="min-h-screen bg-[#fff2eb]">
      <Nav variant="logo" />

      <div className="mx-auto max-w-[1440px] px-8">
        {/* Header */}
        <div className="flex items-center justify-between">
          {/* Left: Back + game title */}
          <div className="flex items-center gap-4">
            <Link
              href="/lobby"
              className="flex items-center gap-1 text-black"
            >
              <img
                src="/icons/arrow-up.svg"
                alt=""
                className="size-4 -rotate-90"
              />
              <span className="font-semibold text-base">{t("back")}</span>
            </Link>
            <div className="flex items-center gap-2">
              <Image
                src={config.icon}
                alt={t(config.titleKey)}
                width={48}
                height={48}
                className="size-12 object-contain"
              />
              <h1 className="font-semibold text-2xl text-black">
                {t(config.titleKey)}
              </h1>
            </div>
          </div>

          {/* Right: Live / Replay tabs */}
          <div className="flex items-center gap-2">
            <button
              onClick={() => setTab("live")}
              className={`flex cursor-pointer items-center gap-2 rounded-full px-4 py-2 font-medium text-base transition-colors ${
                tab === "live"
                  ? "bg-black text-white"
                  : "bg-transparent text-black"
              }`}
            >
              <span
                className="inline-block size-8 shrink-0 bg-current"
                style={{ maskImage: "url(/icons/live.svg)", maskSize: "contain", maskRepeat: "no-repeat", maskPosition: "center", WebkitMaskImage: "url(/icons/live.svg)", WebkitMaskSize: "contain", WebkitMaskRepeat: "no-repeat", WebkitMaskPosition: "center" }}
              />
              {t("live")} ({liveCount})
            </button>
            <button
              onClick={() => setTab("replay")}
              className={`flex cursor-pointer items-center gap-2 rounded-full px-4 py-2 font-medium text-base transition-colors ${
                tab === "replay"
                  ? "bg-black text-white"
                  : "bg-transparent text-black"
              }`}
            >
              <span
                className="inline-block size-8 shrink-0 bg-current"
                style={{ maskImage: "url(/icons/history.svg)", maskSize: "contain", maskRepeat: "no-repeat", maskPosition: "center", WebkitMaskImage: "url(/icons/history.svg)", WebkitMaskSize: "contain", WebkitMaskRepeat: "no-repeat", WebkitMaskPosition: "center" }}
              />
              {t("replay")} ({replayCount.toLocaleString()})
            </button>
          </div>
        </div>

        {/* Table grid */}
        <div
          className="justify-center gap-x-[35px] gap-y-16 px-8 py-16"
          style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, 200px)" }}
        >
          {MOCK_TABLES.map((table, i) => (
            <Link
              key={i}
              href={`/game/${table.id}`}
              className={`relative block overflow-hidden transition-transform hover:scale-105 ${
                config.type === "decorated"
                  ? "h-[128px] w-[200px] rounded-[32px] border-3 border-black bg-[#906c4a]"
                  : "h-[96px] w-[200px]"
              }`}
            >
              <img
                src={config.tableImage}
                alt=""
                className={`pointer-events-none absolute ${
                  config.type === "decorated"
                    ? "inset-[11%_18%] h-[78%] w-[64%] object-contain"
                    : "inset-0 size-full"
                }`}
              />
              <div className="absolute inset-0 flex flex-col items-center justify-center gap-1 text-white">
                <p className="font-medium text-xs">#{table.id}</p>
                <p className="font-semibold text-lg">{table.time}</p>
              </div>
            </Link>
          ))}
        </div>
      </div>
    </main>
  );
}
