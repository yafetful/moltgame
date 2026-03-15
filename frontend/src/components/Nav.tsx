"use client";

import { useTranslations } from "next-intl";
import { Link, usePathname } from "@/i18n/navigation";
import { useState } from "react";
import Image from "next/image";
import LocaleSwitcher from "./LocaleSwitcher";
import { Icon } from "@iconify/react";
import MobileMenu from "./MobileMenu";

const NAV_ITEMS = [
  { key: "home", href: "/", rotate: "-6deg" },
  { key: "lobby", href: "/lobby", rotate: "6deg" },
  { key: "leaderboard", href: "/leaderboard", rotate: "-6deg" },
  { key: "doc", href: "/doc", rotate: "6deg" },
  { key: "dashboard", href: "/dashboard", rotate: "-6deg" },
] as const;

export default function Nav({ variant = "center" }: { variant?: "center" | "logo" }) {
  const t = useTranslations("nav");
  const tHome = useTranslations("home");
  const pathname = usePathname();
  const [hovered, setHovered] = useState<string | null>(null);
  const [menuOpen, setMenuOpen] = useState(false);

  const activeKey = NAV_ITEMS.find((item) =>
    item.href === "/"
      ? pathname === "/" || pathname === ""
      : pathname.startsWith(item.href),
  )?.key;

  const links = (
    <div
      className="flex h-20 items-center gap-4"
      onMouseLeave={() => setHovered(null)}
    >
      {NAV_ITEMS.map((item) => {
        const highlight =
          hovered === item.key ||
          (activeKey === item.key && hovered === null);

        return (
          <Link
            key={item.key}
            href={item.href}
            className="inline-flex items-center justify-center rounded-full border-2 px-4 py-2 transition-all duration-200 ease-out"
            style={{
              borderColor: highlight ? "black" : "transparent",
              transform: highlight ? `rotate(${item.rotate})` : "none",
            }}
            onMouseEnter={() => setHovered(item.key)}
          >
            <span
              className={`text-lg text-black ${highlight ? "font-black" : "font-semibold"}`}
            >
              {t(item.key)}
            </span>
          </Link>
        );
      })}
    </div>
  );

  if (variant === "logo") {
    return (
      <>
        {menuOpen && <MobileMenu onClose={() => setMenuOpen(false)} />}
        <nav className="relative mb-4 flex h-14 items-center justify-between px-4 md:mb-8 md:h-20 md:px-8 md:pt-4">
          {/* Left: hamburger (mobile) / logo (desktop) */}
          <div className="shrink-0">
            <button className="md:hidden" onClick={() => setMenuOpen(true)}>
              <Icon icon="mingcute:menu-line" className="text-black" width={32} />
            </button>
            <Link href="/" className="hidden md:block">
              <Image
                src="/logo/logo-horizontal.png"
                alt="MoltGame"
                width={192}
                height={48}
                className="h-12 w-auto object-contain"
                priority
              />
            </Link>
          </div>

          {/* Center: logo-horizontal (mobile) / nav links (desktop) */}
          <div className="absolute inset-x-0 top-0 bottom-0 flex items-center justify-center pointer-events-none">
            <Link href="/" className="pointer-events-auto md:hidden">
              <Image
                src="/logo/logo-horizontal.png"
                alt="MoltGame"
                width={128}
                height={32}
                className="h-8 w-auto object-contain"
              />
            </Link>
            <div className="pointer-events-auto hidden md:block">{links}</div>
          </div>

          {/* Right: locale switcher */}
          <div className="shrink-0">
            <div className="md:hidden"><LocaleSwitcher compact /></div>
            <div className="hidden md:block"><LocaleSwitcher /></div>
          </div>
        </nav>
      </>
    );
  }

  return (
    <nav className="absolute inset-x-0 top-0 z-50">
      {/* Mobile menu overlay */}
      {menuOpen && <MobileMenu onClose={() => setMenuOpen(false)} />}

      {/* === Mobile nav === */}
      <div className="relative flex flex-col items-center px-4 pt-4 pb-2 md:hidden">
        {/* Hamburger — absolutely positioned top-left */}
        <button className="absolute left-4 top-4 shrink-0" onClick={() => setMenuOpen(true)}>
          <Icon icon="mingcute:menu-line" className="text-black" width={32} />
        </button>
        {/* Lang selector — absolutely positioned top-right */}
        <div className="absolute right-4 top-5">
          <LocaleSwitcher compact />
        </div>
        {/* Centered logo + tagline */}
        <Image
          src="/logo/logo-square.png"
          alt="MoltGame"
          width={96}
          height={96}
          className="size-24 object-contain"
          priority
        />
        <p className="text-2xl text-black" style={{ fontFamily: "begaz, sans-serif" }}>
          {tHome("tagline")}
        </p>
      </div>

      {/* === Desktop nav === */}
      <div className="hidden md:flex items-start justify-center px-8 pt-4">
        {links}
        <div className="absolute right-8 top-11">
          <LocaleSwitcher />
        </div>
      </div>
    </nav>
  );
}
