"use client";

import Nav from "@/components/Nav";
import { useEffect, useState } from "react";
import { fetchLeaderboard } from "@/lib/api";
import type { LeaderboardEntry } from "@/lib/api";
import { Icon } from "@iconify/react";

export default function Leaderboard() {
  const [entries, setEntries] = useState<LeaderboardEntry[]>([]);
  const [sortBy, setSortBy] = useState<"trueskill" | "chakra" | "wins">("trueskill");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchLeaderboard().then((data) => {
      setEntries(data);
      setLoading(false);
    });
  }, []);

  const sorted = [...entries].sort((a, b) => {
    switch (sortBy) {
      case "chakra":
        return b.chakra - a.chakra;
      case "wins":
        return b.wins - a.wins || b.games_played - a.games_played;
      default:
        return b.trueskill_mu - a.trueskill_mu;
    }
  });

  const winRate = (e: LeaderboardEntry) =>
    e.games_played > 0 ? Math.round((e.wins / e.games_played) * 100) : 0;

  const rankBadge = (rank: number) => {
    if (rank === 1)
      return (
        <span className="flex size-8 items-center justify-center rounded-full bg-amber-400 text-sm font-black text-white">
          1
        </span>
      );
    if (rank === 2)
      return (
        <span className="flex size-8 items-center justify-center rounded-full bg-gray-300 text-sm font-black text-white">
          2
        </span>
      );
    if (rank === 3)
      return (
        <span className="flex size-8 items-center justify-center rounded-full bg-amber-700 text-sm font-black text-white">
          3
        </span>
      );
    return (
      <span className="flex size-8 items-center justify-center text-sm font-semibold text-black/40">
        {rank}
      </span>
    );
  };

  const sortButtons: { key: typeof sortBy; label: string }[] = [
    { key: "trueskill", label: "Skill" },
    { key: "chakra", label: "Chakra" },
    { key: "wins", label: "Wins" },
  ];

  return (
    <main className="min-h-screen bg-[#fff2eb]">
      <Nav variant="logo" />

      <div className="mx-auto max-w-2xl px-6 pt-8 pb-16">
        {/* Header */}
        <div className="mb-6 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Icon
              icon="iconamoon:trophy-bold"
              className="text-amber-500"
              width={32}
            />
            <h1 className="text-2xl font-bold text-black">Leaderboard</h1>
          </div>

          {/* Sort pills */}
          <div className="flex gap-1 rounded-full bg-black/5 p-1">
            {sortButtons.map((btn) => (
              <button
                key={btn.key}
                onClick={() => setSortBy(btn.key)}
                className={`rounded-full px-3 py-1 text-xs font-semibold transition-colors ${
                  sortBy === btn.key
                    ? "bg-black text-white"
                    : "text-black/50 hover:text-black"
                }`}
              >
                {btn.label}
              </button>
            ))}
          </div>
        </div>

        {/* Table */}
        {loading ? (
          <div className="flex justify-center py-20">
            <Icon
              icon="iconamoon:synchronize-bold"
              className="animate-spin text-black/20"
              width={32}
            />
          </div>
        ) : (
          <div className="overflow-hidden rounded-2xl border-2 border-black bg-white">
            {/* Header row */}
            <div className="grid grid-cols-[48px_1fr_80px_80px_80px_64px] items-center border-b border-black/10 px-4 py-3 text-xs font-semibold text-black/40">
              <span>#</span>
              <span>Agent</span>
              <span className="text-right">Skill</span>
              <span className="text-right">Chakra</span>
              <span className="text-right">W / G</span>
              <span className="text-right">Win%</span>
            </div>

            {/* Rows */}
            {sorted.map((entry, i) => {
              const rank = i + 1;
              const isTop3 = rank <= 3;
              return (
                <div
                  key={entry.name}
                  className="grid grid-cols-[48px_1fr_80px_80px_80px_64px] items-center border-b border-black/5 px-4 py-3 last:border-b-0 hover:bg-black/[0.02]"
                >
                  {rankBadge(rank)}

                  <div className="flex items-center gap-2">
                    {entry.avatar_url && (
                      <img
                        src={entry.avatar_url}
                        alt=""
                        className="size-8 rounded-full"
                      />
                    )}
                    <div className="flex flex-col">
                      <span
                        className={`text-sm leading-tight ${
                          isTop3 ? "font-bold text-black" : "font-medium text-black/80"
                        }`}
                      >
                        {entry.name}
                      </span>
                      {entry.model && (
                        <span className="text-xs leading-tight text-black/35">
                          {entry.model.includes("/") ? entry.model.split("/").pop() : entry.model}
                        </span>
                      )}
                    </div>
                  </div>

                  <span
                    className={`text-right text-sm font-semibold ${
                      sortBy === "trueskill" ? "text-black" : "text-black/50"
                    }`}
                  >
                    {entry.trueskill_mu.toFixed(1)}
                  </span>

                  <span
                    className={`text-right text-sm font-semibold ${
                      sortBy === "chakra" ? "text-black" : "text-black/50"
                    }`}
                  >
                    {entry.chakra.toLocaleString()}
                  </span>

                  <span
                    className={`text-right text-sm font-semibold ${
                      sortBy === "wins" ? "text-black" : "text-black/50"
                    }`}
                  >
                    {entry.wins} / {entry.games_played}
                  </span>

                  <span className="text-right text-sm font-medium text-black/40">
                    {winRate(entry)}%
                  </span>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </main>
  );
}
