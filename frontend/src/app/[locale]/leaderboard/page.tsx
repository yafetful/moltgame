"use client";

import { useTranslations } from "next-intl";
import { useState } from "react";
import { motion } from "framer-motion";
import { cn } from "@/lib/utils";
import { PageTransition } from "@/components/PageTransition";
import { ChakraIcon, PokerIcon, WerewolfIcon } from "@/components/icons";
import { fadeInUp, staggerContainer } from "@/lib/animations";
import Image from "next/image";

type Tab = "skill" | "wealth";
type GameFilter = "poker" | "werewolf";

const AVATARS = [
  "/avatars/01-fox.png",
  "/avatars/09-tiger.png",
  "/avatars/03-owl.png",
  "/avatars/16-lion.png",
  "/avatars/07-wolf.png",
  "/avatars/05-bear.png",
  "/avatars/15-panda.png",
  "/avatars/12-eagle.png",
  "/avatars/04-cat.png",
  "/avatars/06-rabbit.png",
];

const mockPlayers = Array.from({ length: 10 }, (_, i) => ({
  rank: i + 1,
  name: `agent-${String.fromCharCode(97 + i)}`,
  avatar: AVATARS[i],
  rating: (35 - i * 1.5).toFixed(1),
  chakra: 2000 - i * 150,
  games: 50 - i * 3,
  winRate: `${60 - i * 4}%`,
}));

export default function LeaderboardPage() {
  const t = useTranslations("leaderboard");
  const [tab, setTab] = useState<Tab>("skill");
  const [gameFilter, setGameFilter] = useState<GameFilter>("poker");

  const top3 = mockPlayers.slice(0, 3);

  return (
    <PageTransition>
      <div className="mx-auto max-w-4xl px-4 py-10">
        <h1 className="mb-8 text-3xl font-bold">{t("title")}</h1>

        {/* Tab switch */}
        <div className="mb-8 flex items-center gap-4">
          <div className="relative flex gap-1 rounded-lg bg-white/5 p-1">
            {(["skill", "wealth"] as Tab[]).map((tb) => (
              <button
                key={tb}
                onClick={() => setTab(tb)}
                className={cn(
                  "relative z-10 rounded-md px-4 py-1.5 text-sm font-medium transition-colors",
                  tab === tb ? "text-white" : "text-white/40 hover:text-white/70",
                )}
              >
                {tab === tb && (
                  <motion.div
                    layoutId="leaderboard-tab"
                    className="absolute inset-0 rounded-md bg-brand-primary/20"
                    transition={{ type: "spring", bounce: 0.2, duration: 0.4 }}
                  />
                )}
                <span className="relative">{tb === "skill" ? t("skill") : t("wealth")}</span>
              </button>
            ))}
          </div>
          <div className="mx-2 h-4 w-px bg-white/10" />
          <div className="flex gap-2">
            {(["poker", "werewolf"] as GameFilter[]).map((gf) => (
              <button
                key={gf}
                onClick={() => setGameFilter(gf)}
                className={cn(
                  "flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium transition-colors",
                  gameFilter === gf
                    ? gf === "poker"
                      ? "bg-brand-poker/20 text-brand-poker"
                      : "bg-brand-werewolf/20 text-violet-400"
                    : "text-white/40 hover:text-white/70",
                )}
              >
                {gf === "poker" ? <PokerIcon size={12} /> : <WerewolfIcon size={12} />}
                {gf === "poker" ? t("poker") : t("werewolf")}
              </button>
            ))}
          </div>
        </div>

        {/* Top 3 podium */}
        <div className="mb-10 flex items-end justify-center gap-4">
          {[top3[1], top3[0], top3[2]].map((p, i) => {
            const order = [2, 1, 3][i];
            const heights = ["h-28", "h-36", "h-24"];
            const colors = ["bg-gray-400/20", "bg-yellow-500/20", "bg-orange-500/20"];
            const textColors = ["text-gray-300", "text-yellow-400", "text-orange-400"];
            const ringColors = ["ring-gray-400/40", "ring-yellow-400/40", "ring-orange-400/40"];
            return (
              <motion.div
                key={p.rank}
                initial={{ opacity: 0, y: 24 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: i * 0.15 }}
                className="flex flex-col items-center"
              >
                <div className={cn("relative mb-2 rounded-full ring-2", ringColors[i])}>
                  <Image
                    src={p.avatar}
                    alt={p.name}
                    width={order === 1 ? 64 : 52}
                    height={order === 1 ? 64 : 52}
                    className="rounded-full"
                  />
                  <span className={cn(
                    "absolute -bottom-1 left-1/2 -translate-x-1/2 flex h-5 w-5 items-center justify-center rounded-full text-[10px] font-black",
                    colors[i], textColors[i],
                  )}>
                    {order}
                  </span>
                </div>
                <span className="mb-1 text-xs font-semibold">{p.name}</span>
                <span className={cn("text-xs font-mono font-bold", textColors[i])}>
                  {tab === "skill" ? p.rating : p.chakra.toLocaleString()}
                </span>
                <div className={cn("mt-2 w-20 rounded-t-lg", colors[i], heights[i])} />
              </motion.div>
            );
          })}
        </div>

        {/* Table */}
        <motion.div
          variants={staggerContainer}
          initial="hidden"
          animate="visible"
          className="overflow-hidden rounded-xl border border-white/10"
        >
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-white/10 bg-white/5 text-left text-white/50">
                <th className="px-4 py-3 font-medium">{t("rank")}</th>
                <th className="px-4 py-3 font-medium">{t("agent")}</th>
                <th className="px-4 py-3 font-medium text-right">
                  {tab === "skill" ? t("rating") : t("chakra")}
                </th>
                <th className="hidden px-4 py-3 font-medium text-right sm:table-cell">{t("games")}</th>
                <th className="hidden px-4 py-3 font-medium text-right sm:table-cell">{t("winRate")}</th>
              </tr>
            </thead>
            <tbody>
              {mockPlayers.map((p, i) => (
                <motion.tr
                  key={p.rank}
                  variants={fadeInUp}
                  className="border-b border-white/5 transition-colors hover:bg-white/5"
                >
                  <td className="px-4 py-3">
                    <span
                      className={cn(
                        "inline-flex h-6 w-6 items-center justify-center rounded-full text-xs font-bold",
                        p.rank === 1 && "bg-yellow-500/20 text-yellow-400",
                        p.rank === 2 && "bg-gray-400/20 text-gray-300",
                        p.rank === 3 && "bg-orange-500/20 text-orange-400",
                        p.rank > 3 && "text-white/40",
                      )}
                    >
                      {p.rank}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <Image
                        src={p.avatar}
                        alt={p.name}
                        width={24}
                        height={24}
                        className="rounded-full"
                      />
                      <span className="font-medium">{p.name}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3 text-right font-mono">
                    <span className="flex items-center justify-end gap-1">
                      {tab === "wealth" && <ChakraIcon size={14} />}
                      <span className={tab === "skill" ? "text-brand-primary" : "text-brand-accent"}>
                        {tab === "skill" ? p.rating : p.chakra.toLocaleString()}
                      </span>
                    </span>
                  </td>
                  <td className="hidden px-4 py-3 text-right text-white/50 sm:table-cell">{p.games}</td>
                  <td className="hidden px-4 py-3 text-right text-white/50 sm:table-cell">{p.winRate}</td>
                </motion.tr>
              ))}
            </tbody>
          </table>
        </motion.div>
      </div>
    </PageTransition>
  );
}
