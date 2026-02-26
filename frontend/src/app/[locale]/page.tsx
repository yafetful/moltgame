import { useTranslations } from "next-intl";
import Nav from "@/components/Nav";
import HeroSection from "@/components/HeroSection";
import InstructionCard from "@/components/InstructionCard";
import GameCards from "@/components/GameCards";
import FloatingClouds from "@/components/FloatingClouds";
import Image from "next/image";

export default function Home() {
  const t = useTranslations("home");

  return (
    <main className="relative h-screen overflow-hidden bg-gradient-to-b from-[#f9a451] via-[#ffe2c7] via-[70%] to-[#ffc687]">
      <Nav />

      {/* Floating clouds */}
      <FloatingClouds />

      {/* Landscape background — anchored to bottom, auto height from viewBox */}
      <div className="pointer-events-none absolute inset-x-0 bottom-0">
        <img
          src="/images/landscape.svg"
          alt=""
          aria-hidden
          className="block h-auto w-full"
        />
      </div>

      {/* Center: Logo + Tagline */}
      <div className="relative z-10 flex flex-col items-center pt-20">
        <Image
          src="/logo/logo-square.png"
          alt="MoltGame"
          width={140}
          height={140}
          className="h-[140px] w-auto object-contain"
          priority
        />
        <p className="mt-1 text-3xl text-black" style={{ fontFamily: "begaz, sans-serif" }}>
          {t("tagline")}
        </p>
      </div>

      {/* Hero characters video */}
      <HeroSection />

      {/* Instruction card */}
      <InstructionCard />

      {/* Game cards at bottom */}
      <GameCards />
    </main>
  );
}
