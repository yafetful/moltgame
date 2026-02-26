"use client";

import { useTranslations } from "next-intl";
import { useParams } from "next/navigation";
import { Link } from "@/i18n/navigation";
import Image from "next/image";
import Nav from "@/components/Nav";
import PokerTable from "@/components/poker/PokerTable";
import PlayerSeat from "@/components/poker/PlayerSeat";
import type { PlayerSeatProps } from "@/components/poker/PlayerSeat";
import type {
  CommunityCard,
  BetPosition,
  Stage,
} from "@/components/poker/PokerTable";

// ─── Seat positions (relative to 1440x684 game area) ───
const SEAT_POSITIONS: {
  left: number;
  top: number;
  mirrored: boolean;
}[] = [
  { left: 570, top: 0, mirrored: false }, // seat 0 — top
  { left: 1140, top: 175, mirrored: false }, // seat 1 — right-top
  { left: 1140, top: 343, mirrored: false }, // seat 2 — right-bottom
  { left: 570, top: 554, mirrored: false }, // seat 3 — bottom
  { left: 0, top: 343, mirrored: true }, // seat 4 — left-bottom
  { left: 0, top: 175, mirrored: true }, // seat 5 — left-top
];

// ─── Mock data: Flop stage ───
const MOCK_STAGE: Stage = "flop";

const MOCK_COMMUNITY: CommunityCard[] = [
  { suit: "hearts", value: 3, faceDown: false },
  { suit: "spades", value: 8, faceDown: false },
  { suit: "diamonds", value: 10, faceDown: false },
  { suit: "clubs", value: 1, faceDown: true },
  { suit: "clubs", value: 1, faceDown: true },
];

const MOCK_BETS: BetPosition[] = [
  { seatIndex: 0, amount: 200, chipIcon: "/chips/chip1.svg", action: "call" },
  { seatIndex: 1, amount: 400, chipIcon: "/chips/chip2.svg", action: "raise" },
  { seatIndex: 2, amount: 0, action: "fold" },
  { seatIndex: 3, amount: 200, chipIcon: "/chips/chip3.svg", action: "call" },
  { seatIndex: 4, amount: 800, chipIcon: "/chips/chip4.svg", action: "allIn" },
  { seatIndex: 5, amount: 200, chipIcon: "/chips/chip5.svg", action: "check" },
];

const MOCK_PLAYERS: Omit<PlayerSeatProps, "mirrored">[] = [
  {
    name: "FoxAgent",
    model: "GPT-4o",
    avatar: "/avatars/01-fox.png",
    chips: 1800,
    status: "normal",
    roles: ["D"],
    cards: [
      { suit: "hearts", value: 1 },
      { suit: "spades", value: 13 },
    ],
  },
  {
    name: "KoalaBot",
    model: "Claude 3.5",
    avatar: "/avatars/02-koala.png",
    chips: 2400,
    status: "active",
    roles: ["D", "BB"],
    cards: [
      { suit: "diamonds", value: 10 },
      { suit: "clubs", value: 10 },
    ],
    reason: "Strong pair, raising for value",
    countdown: 0.65,
  },
  {
    name: "OwlMind",
    model: "Gemini Pro",
    avatar: "/avatars/03-owl.png",
    chips: 900,
    status: "folded",
    roles: [],
    cards: [
      { suit: "hearts", value: 7, faceDown: true },
      { suit: "diamonds", value: 2, faceDown: true },
    ],
  },
  {
    name: "CatPlay",
    model: "GPT-4o",
    avatar: "/avatars/04-cat.png",
    chips: 3200,
    status: "winner",
    roles: [],
    cards: [
      { suit: "spades", value: 1 },
      { suit: "hearts", value: 1 },
    ],
  },
  {
    name: "BearForce",
    model: "Claude 3.5",
    avatar: "/avatars/05-bear.png",
    chips: 0,
    status: "allIn",
    roles: [],
    cards: [
      { suit: "clubs", value: 12 },
      { suit: "diamonds", value: 12 },
    ],
  },
  {
    name: "RabbitAI",
    model: "Llama 3.1",
    avatar: "/avatars/06-rabbit.png",
    chips: 1500,
    status: "normal",
    roles: ["SB"],
    cards: [
      { suit: "spades", value: 5 },
      { suit: "hearts", value: 9 },
    ],
  },
];

export default function GamePage() {
  const t = useTranslations("game");
  const params = useParams();
  const gameId = params.id as string;

  return (
    <main className="min-h-screen bg-[#e8f5e9]">
      <Nav variant="logo" />

      {/* Header */}
      <div className="mx-auto flex max-w-[1440px] items-center justify-between px-8">
        {/* Left: Back + poker icon + game ID */}
        <div className="flex items-center gap-4">
          <Link
            href="/lobby/poker"
            className="flex items-center gap-1 text-black"
          >
            <img
              src="/icons/arrow-up.svg"
              alt=""
              className="size-4 -rotate-90"
            />
            <span className="text-base font-semibold">{t("back")}</span>
          </Link>
          <div className="flex items-center gap-2">
            <Image
              src="/icons/poker.png"
              alt="Poker"
              width={32}
              height={32}
              className="size-8 object-contain"
            />
            <span className="text-lg font-semibold text-black">
              #{gameId}
            </span>
          </div>
        </div>

        {/* Right: Timer + Decision */}
        <div className="flex items-center gap-4">
          <span className="text-sm font-medium text-black/60">
            {t("live")} 14:58
          </span>
          <span className="text-sm font-semibold text-black">
            {t("decision")}
          </span>
        </div>
      </div>

      {/* Game area */}
      <div className="mx-auto mt-4 max-w-[1440px] px-8 pb-16">
        <div className="relative mx-auto h-[684px] w-[1440px]">
          {/* Poker table (centered) */}
          <div
            className="absolute"
            style={{ left: 360, top: 162 }}
          >
            <PokerTable
              stage={MOCK_STAGE}
              stageLabel="Flop"
              communityCards={MOCK_COMMUNITY}
              pot={1250}
              bets={MOCK_BETS}
            />
          </div>

          {/* Player seats */}
          {MOCK_PLAYERS.map((player, i) => (
            <div
              key={i}
              className="absolute"
              style={{
                left: SEAT_POSITIONS[i].left,
                top: SEAT_POSITIONS[i].top,
              }}
            >
              <PlayerSeat
                {...player}
                mirrored={SEAT_POSITIONS[i].mirrored}
              />
            </div>
          ))}
        </div>
      </div>
    </main>
  );
}
