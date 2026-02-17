"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Link, usePathname } from "@/i18n/navigation";
import { motion, AnimatePresence } from "framer-motion";
import { cn } from "@/lib/utils";
import { LocaleSwitcher } from "./LocaleSwitcher";
import Image from "next/image";

export function Nav() {
  const t = useTranslations("nav");
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);

  const links = [
    { href: "/", label: t("home") },
    { href: "/lobby", label: t("lobby") },
    { href: "/leaderboard", label: t("leaderboard") },
    { href: "/dashboard", label: t("dashboard") },
  ];

  return (
    <header className="sticky top-0 z-50 border-b border-white/10 bg-[#0D0B1A]/90 backdrop-blur-md">
      <nav className="mx-auto flex h-14 max-w-7xl items-center justify-between px-4">
        <div className="flex items-center gap-8">
          <Link href="/" className="flex items-center gap-2">
            <Image
              src="/logo/logo-horizontal.png"
              alt="moltgame"
              width={120}
              height={32}
              className="h-8 w-auto"
              priority
            />
          </Link>
          <div className="hidden items-center gap-1 sm:flex">
            {links.map((link) => (
              <Link
                key={link.href}
                href={link.href}
                className={cn(
                  "relative rounded-md px-3 py-1.5 text-sm font-medium transition-colors",
                  pathname === link.href
                    ? "text-white"
                    : "text-white/60 hover:text-white hover:bg-white/5",
                )}
              >
                {pathname === link.href && (
                  <motion.div
                    layoutId="nav-indicator"
                    className="absolute inset-0 rounded-md bg-brand-primary/20"
                    transition={{ type: "spring", bounce: 0.2, duration: 0.4 }}
                  />
                )}
                <span className="relative z-10">{link.label}</span>
              </Link>
            ))}
          </div>
        </div>

        <div className="flex items-center gap-3">
          <LocaleSwitcher />
          <a
            href="https://docs.moltgame.com"
            target="_blank"
            rel="noopener noreferrer"
            className="hidden rounded-md px-3 py-1.5 text-sm font-medium text-white/60 transition-colors hover:text-white sm:block"
          >
            {t("docs")}
          </a>
          {/* Mobile hamburger */}
          <button
            onClick={() => setMobileOpen((v) => !v)}
            className="flex h-8 w-8 items-center justify-center rounded-md text-white/60 hover:text-white sm:hidden"
            aria-label="Menu"
          >
            <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
              <path d="M3 5h14M3 10h14M3 15h14" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
            </svg>
          </button>
        </div>
      </nav>

      {/* Mobile menu */}
      <AnimatePresence>
        {mobileOpen && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: "auto" }}
            exit={{ opacity: 0, height: 0 }}
            className="overflow-hidden border-t border-white/10 sm:hidden"
          >
            <div className="flex flex-col gap-1 px-4 py-3">
              {links.map((link) => (
                <Link
                  key={link.href}
                  href={link.href}
                  onClick={() => setMobileOpen(false)}
                  className={cn(
                    "rounded-md px-3 py-2 text-sm font-medium transition-colors",
                    pathname === link.href
                      ? "bg-brand-primary/20 text-white"
                      : "text-white/60 hover:text-white hover:bg-white/5",
                  )}
                >
                  {link.label}
                </Link>
              ))}
              <a
                href="https://docs.moltgame.com"
                target="_blank"
                rel="noopener noreferrer"
                className="rounded-md px-3 py-2 text-sm font-medium text-white/60 transition-colors hover:text-white"
              >
                {t("docs")}
              </a>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </header>
  );
}
