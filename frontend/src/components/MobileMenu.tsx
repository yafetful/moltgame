"use client";

import { useTranslations } from "next-intl";
import { Link, usePathname } from "@/i18n/navigation";
import Image from "next/image";

const MENU_ITEMS = [
  { key: "home", href: "/", rotate: "-6deg" },
  { key: "lobby", href: "/lobby", rotate: "6deg" },
  { key: "leaderboard", href: "/leaderboard", rotate: "-6deg" },
  { key: "doc", href: "/doc", rotate: "6deg" },
  { key: "dashboard", href: "/dashboard", rotate: "-6deg" },
] as const;

export default function MobileMenu({ onClose }: { onClose: () => void }) {
  const t = useTranslations("nav");
  const tHome = useTranslations("home");
  const pathname = usePathname();

  const activeKey = MENU_ITEMS.find((item) =>
    item.href === "/"
      ? pathname === "/" || pathname === ""
      : pathname.startsWith(item.href),
  )?.key;

  return (
    <div className="fixed inset-0 z-[200] flex flex-col bg-[#fff2eb] pb-16">
      {/* Close button */}
      <button
        onClick={onClose}
        className="absolute right-4 top-4 shrink-0"
        aria-label="Close menu"
      >
        <img src="/icons/close-circle.svg" alt="" className="size-8" />
      </button>

      {/* Nav links — vertically centered */}
      <div className="flex flex-1 flex-col items-center justify-center gap-8">
        {MENU_ITEMS.map((item) => {
          const isActive = activeKey === item.key;
          const content = (
            <span
              className={`text-2xl text-black ${isActive ? "font-black" : "font-semibold"}`}
            >
              {t(item.key)}
            </span>
          );

          const wrapperClass = `inline-flex items-center justify-center rounded-full px-4 py-2 ${
            isActive ? "border-2 border-black" : ""
          }`;
          const wrapperStyle = isActive ? { transform: `rotate(${item.rotate})` } : undefined;

          return (
            <Link
              key={item.key}
              href={item.href}
              onClick={onClose}
              className={wrapperClass}
              style={wrapperStyle}
            >
              {content}
            </Link>
          );
        })}
      </div>

      {/* Bottom: logo + tagline */}
      <div className="flex flex-col items-center pb-4">
        <Image
          src="/logo/logo-square.png"
          alt="MoltGame"
          width={96}
          height={96}
          className="size-24 object-contain"
        />
        <p className="text-2xl text-black" style={{ fontFamily: "begaz, sans-serif" }}>
          {tHome("tagline")}
        </p>
      </div>
    </div>
  );
}
