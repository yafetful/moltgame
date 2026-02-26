"use client";

import { useTranslations } from "next-intl";
import { Link, usePathname } from "@/i18n/navigation";
import { useState } from "react";
import Image from "next/image";
import LocaleSwitcher from "./LocaleSwitcher";

const NAV_ITEMS = [
  { key: "home", href: "/", rotate: "-6deg" },
  { key: "lobby", href: "/lobby", rotate: "6deg" },
  { key: "leaderboard", href: "/leaderboard", rotate: "-6deg" },
  { key: "dashboard", href: "/dashboard", rotate: "6deg" },
] as const;

export default function Nav({ variant = "center" }: { variant?: "center" | "logo" }) {
  const t = useTranslations("nav");
  const pathname = usePathname();
  const [hovered, setHovered] = useState<string | null>(null);

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
      <nav className="mb-8 flex h-20 items-center justify-between px-8 pt-4">
        <Link href="/" className="shrink-0">
          <Image
            src="/logo/logo-horizontal.png"
            alt="MoltGame"
            width={192}
            height={48}
            className="h-12 w-auto object-contain"
            priority
          />
        </Link>
        <div className="absolute inset-x-0 top-0 flex justify-center pointer-events-none">
          <div className="pointer-events-auto">{links}</div>
        </div>
        <div className="shrink-0">
          <LocaleSwitcher />
        </div>
      </nav>
    );
  }

  return (
    <nav className="absolute inset-x-0 top-0 z-50 flex items-start justify-center px-8 pt-4">
      {links}
      <div className="absolute right-8 top-11">
        <LocaleSwitcher />
      </div>
    </nav>
  );
}
