"use client";

import { useTranslations } from "next-intl";
import { useEffect, useState } from "react";
import { motion } from "framer-motion";
import { API, type LiveGame, type RecentGame } from "@/lib/api";
import { Link } from "@/i18n/navigation";
import { cn } from "@/lib/utils";
import { PageTransition } from "@/components/PageTransition";
import { PokerIcon, WerewolfIcon } from "@/components/icons";
import { FloatingCharacter } from "@/components/FloatingCharacter";
import { fadeInUp, staggerContainer } from "@/lib/animations";

type Filter = "all" | "poker" | "werewolf";

export default function LobbyPage() {
  const t = useTranslations("lobby");
  const [games, setGames] = useState<LiveGame[]>([]);
  const [recentGames, setRecentGames] = useState<RecentGame[]>([]);
  const [filter, setFilter] = useState<Filter>("all");
  const [queue, setQueue] = useState<Record<string, number>>({});

  useEffect(() => {
    const load = () => {
      API.liveGames().then(setGames).catch(() => {});
      API.queueStatus().then(setQueue).catch(() => {});
    };
    load();
    const interval = setInterval(load, 5000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    API.recentGames().then(setRecentGames).catch(() => {});
  }, []);

  const filtered = filter === "all" ? games : games.filter((g) => g.game_type === filter);

  return (
    <PageTransition>
      <div className="mx-auto max-w-7xl px-4 py-10">
        <h1 className="mb-8 text-3xl font-bold">{t("title")}</h1>

        {/* Queue status */}
        <div className="mb-6 flex items-center gap-4 text-sm text-white/50">
          <span>{t("queueStatus")}:</span>
          {Object.entries(queue).map(([type, count]) => (
            <span key={type} className="rounded-full bg-white/10 px-3 py-0.5">
              {type}: {count} {t("inQueue")}
            </span>
          ))}
        </div>

        {/* Filter pills */}
        <div className="mb-6 flex gap-2">
          {(["all", "poker", "werewolf"] as Filter[]).map((f) => (
            <button
              key={f}
              onClick={() => setFilter(f)}
              className={cn(
                "flex items-center gap-1.5 rounded-full px-4 py-1.5 text-sm font-medium transition-colors",
                filter === f
                  ? f === "poker"
                    ? "bg-brand-poker/20 text-brand-poker"
                    : f === "werewolf"
                      ? "bg-brand-werewolf/20 text-violet-400"
                      : "bg-brand-primary/20 text-brand-primary"
                  : "text-white/40 hover:text-white/70 hover:bg-white/5",
              )}
            >
              {f === "poker" && <PokerIcon size={14} />}
              {f === "werewolf" && <WerewolfIcon size={14} />}
              {f === "all" ? t("filterAll") : f === "poker" ? t("filterPoker") : t("filterWerewolf")}
            </button>
          ))}
        </div>

        {/* Game list */}
        {filtered.length === 0 ? (
          <div className="flex flex-col items-center py-24">
            <FloatingCharacter src="/avatars/03-owl.png" size={80} className="mb-4 opacity-40" />
            <p className="text-white/30">{t("noGames")}</p>
            <Link
              href="/"
              className="mt-4 rounded-lg bg-brand-primary/20 px-4 py-2 text-sm font-medium text-brand-primary hover:bg-brand-primary/30"
            >
              {t("filterAll")}
            </Link>
          </div>
        ) : (
          <motion.div
            variants={staggerContainer}
            initial="hidden"
            animate="visible"
            className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3"
          >
            {filtered.map((game) => (
              <motion.div key={game.game_id} variants={fadeInUp}>
                <Link
                  href={`/game/${game.game_id}`}
                  className={cn(
                    "group block rounded-xl border border-white/10 bg-white/5 p-6 transition-all duration-300",
                    game.game_type === "poker"
                      ? "hover:border-brand-poker/30 hover:shadow-lg hover:shadow-brand-poker/5"
                      : "hover:border-brand-werewolf/30 hover:shadow-lg hover:shadow-brand-werewolf/5",
                  )}
                >
                  <div className="mb-3 flex items-center justify-between">
                    <span className="flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider text-white/40">
                      {game.game_type === "poker" ? <PokerIcon size={14} /> : <WerewolfIcon size={14} />}
                      {game.game_type === "poker" ? "Poker" : "Werewolf"}
                    </span>
                    <span className="flex items-center gap-1.5">
                      <span className="relative flex h-2 w-2">
                        <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-red-400 opacity-75" />
                        <span className="relative inline-flex h-2 w-2 rounded-full bg-red-500" />
                      </span>
                      <span className={cn(
                        "rounded-full px-2 py-0.5 text-xs",
                        game.game_type === "poker" ? "bg-brand-poker/20 text-brand-poker" : "bg-brand-werewolf/20 text-violet-400",
                      )}>
                        {game.phase}
                      </span>
                    </span>
                  </div>
                  <div className="mb-4 text-sm text-white/60">
                    {t("players")}: {game.player_count}
                    {game.hand_num ? ` · ${t("hand")} #${game.hand_num}` : ""}
                    {game.day ? ` · ${t("day")} ${game.day}` : ""}
                  </div>
                  <div className={cn(
                    "text-right text-xs font-medium opacity-0 transition-opacity group-hover:opacity-100",
                    game.game_type === "poker" ? "text-brand-poker" : "text-violet-400",
                  )}>
                    {t("spectate")} →
                  </div>
                </Link>
              </motion.div>
            ))}
          </motion.div>
        )}

        {/* Recent / Finished Games */}
        <div className="mt-12">
          <h2 className="mb-6 text-xl font-bold">{t("recentGames")}</h2>
          {recentGames.length === 0 ? (
            <p className="py-10 text-center text-white/30">{t("noRecentGames")}</p>
          ) : (
            <motion.div
              variants={staggerContainer}
              initial="hidden"
              animate="visible"
              className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3"
            >
              {recentGames.map((game) => (
                <motion.div key={game.game_id} variants={fadeInUp}>
                  <Link
                    href={`/game/${game.game_id}/replay`}
                    className="group block rounded-xl border border-white/10 bg-white/5 p-6 transition-all duration-300 hover:border-brand-accent/30"
                  >
                    <div className="mb-3 flex items-center justify-between">
                      <span className="flex items-center gap-1.5 text-xs font-medium uppercase tracking-wider text-white/40">
                        {game.game_type === "poker" ? <PokerIcon size={14} /> : <WerewolfIcon size={14} />}
                        {game.game_type === "poker" ? "Poker" : "Werewolf"}
                      </span>
                      <span className="rounded-full bg-white/10 px-2 py-0.5 text-xs text-white/40">
                        {t("finished")}
                      </span>
                    </div>
                    <div className="mb-2 text-sm text-white/60">
                      {t("players")}: {game.player_count}
                      {game.winner_name && (
                        <span>
                          {" · "}
                          {t("winner")}: {game.winner_name}
                        </span>
                      )}
                    </div>
                    {game.finished_at && (
                      <div className="mb-3 text-xs text-white/30">
                        {new Date(game.finished_at).toLocaleString()}
                      </div>
                    )}
                    <div className="text-right text-xs font-medium text-brand-accent opacity-0 transition-opacity group-hover:opacity-100">
                      {t("replay")} →
                    </div>
                  </Link>
                </motion.div>
              ))}
            </motion.div>
          )}
        </div>
      </div>
    </PageTransition>
  );
}
