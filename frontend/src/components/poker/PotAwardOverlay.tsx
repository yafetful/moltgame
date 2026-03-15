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

// Mobile seat targets (in 402×810 game container coordinate system)
const MOBILE_SEAT_TARGETS: { x: number; y: number }[] = [
  { x: 167, y: 116 },  // seat 0
  { x: 319, y: 234 },  // seat 1
  { x: 319, y: 483 },  // seat 2
  { x: 165, y: 696 },  // seat 3
  { x: 86,  y: 483 },  // seat 4
  { x: 85,  y: 234 },  // seat 5
];

const MOBILE_TABLE_CENTER = { x: 201, y: 384 };

export default function PotAwardOverlay({
  winners,
  mobile = false,
}: {
  winners: PotAwardWinner[];
  mobile?: boolean;
}) {
  const targets = mobile ? MOBILE_SEAT_TARGETS : SEAT_TARGETS;
  const center = mobile ? MOBILE_TABLE_CENTER : TABLE_CENTER;

  return (
    <AnimatePresence>
      {winners.map((w) => {
        const target = targets[w.seat] ?? center;
        return (
          <motion.div
            key={`pot-award-${w.seat}`}
            className="pointer-events-none absolute z-50 rounded-full bg-[#00d74b] px-3 py-1 text-sm font-bold text-white shadow-lg"
            initial={{
              x: center.x,
              y: center.y,
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
