"use client";

import { motion, AnimatePresence } from "framer-motion";

interface PotAwardWinner {
  seat: number;
  amount: number;
}

// Target positions for each seat's chips badge (inside 1440x684 game area)
const SEAT_TARGETS: { x: number; y: number }[] = [
  { x: 820, y: 82 },   // seat 0
  { x: 1390, y: 257 }, // seat 1
  { x: 1390, y: 425 }, // seat 2
  { x: 820, y: 636 },  // seat 3
  { x: 50, y: 425 },   // seat 4
  { x: 50, y: 257 },   // seat 5
];

const TABLE_CENTER = { x: 720, y: 372 };

export default function PotAwardOverlay({
  winners,
}: {
  winners: PotAwardWinner[];
}) {
  return (
    <AnimatePresence>
      {winners.map((w) => {
        const target = SEAT_TARGETS[w.seat] ?? TABLE_CENTER;
        return (
          <motion.div
            key={`pot-award-${w.seat}`}
            className="pointer-events-none absolute z-50 rounded-full bg-[#00d74b] px-3 py-1 text-sm font-bold text-white shadow-lg"
            initial={{
              x: TABLE_CENTER.x,
              y: TABLE_CENTER.y,
              opacity: 1,
              scale: 1.2,
            }}
            animate={{
              x: target.x,
              y: target.y,
              opacity: 0,
              scale: 1,
            }}
            exit={{ opacity: 0 }}
            transition={{
              duration: 1.2,
              ease: "easeInOut",
            }}
          >
            +${w.amount.toLocaleString()}
          </motion.div>
        );
      })}
    </AnimatePresence>
  );
}
