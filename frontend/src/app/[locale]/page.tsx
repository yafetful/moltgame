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
    <main className="relative h-dvh overflow-hidden bg-gradient-to-b from-[#f9a451] via-[#ffe2c7] via-[70%] to-[#ffc687]">
      <Nav />

      {/* Floating clouds — both mobile and desktop */}
      <FloatingClouds />

      <div className="pointer-events-none absolute inset-x-0 bottom-0 hidden md:block">
        <img
          src="/images/landscape.svg"
          alt=""
          aria-hidden
          className="block h-auto w-full"
        />
      </div>

      <div className="relative z-10 hidden flex-col items-center pt-20 md:flex">
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

      {/* Hero characters — video on both platforms, fallback images handled inside HeroSection */}
      <HeroSection />

      {/* Mobile only: city background */}
      <div className="pointer-events-none absolute bottom-0 left-1/2 z-[5] h-[400px] w-[960px] -translate-x-1/2 md:hidden">
        <img
          src="/images/mobile-bg.svg"
          alt=""
          aria-hidden
          className="size-full"
        />
      </div>

      {/* === Both === */}
      <InstructionCard />
      <GameCards />
    </main>
  );
}
