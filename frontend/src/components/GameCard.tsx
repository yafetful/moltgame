"use client";

import { motion } from "framer-motion";
import Image from "next/image";
import { cn } from "@/lib/utils";

interface GameCardProps {
  type: "poker" | "werewolf";
  children: React.ReactNode;
  className?: string;
  isLive?: boolean;
  onClick?: () => void;
}

export function GameCard({ type, children, className, isLive, onClick }: GameCardProps) {
  const borderColor = type === "poker" ? "hover:border-brand-poker/40" : "hover:border-brand-werewolf/40";
  const glowColor = type === "poker" ? "hover:shadow-brand-poker/10" : "hover:shadow-brand-werewolf/10";

  return (
    <motion.div
      whileHover={{ scale: 1.02, rotateY: 2 }}
      whileTap={{ scale: 0.98 }}
      onClick={onClick}
      className={cn(
        "group relative cursor-pointer rounded-xl border border-white/10 bg-white/5 p-6 transition-all duration-300",
        borderColor,
        `hover:shadow-lg ${glowColor}`,
        className,
      )}
      style={{ transformStyle: "preserve-3d" }}
    >
      {isLive && (
        <span className="absolute right-4 top-4 flex items-center gap-1.5">
          <span className="relative flex h-2 w-2">
            <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-red-400 opacity-75" />
            <span className="relative inline-flex h-2 w-2 rounded-full bg-red-500" />
          </span>
          <span className="text-[10px] font-bold uppercase tracking-wider text-red-400">Live</span>
        </span>
      )}
      <div className="mb-3">
        <Image
          src={type === "poker" ? "/icons/poker.png" : "/icons/werewolves.png"}
          alt={type}
          width={32}
          height={32}
          className="h-8 w-8"
        />
      </div>
      {children}
    </motion.div>
  );
}
