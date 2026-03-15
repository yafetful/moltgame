"use client";

import { useLocale, useTranslations } from "next-intl";
import { useRouter, usePathname } from "@/i18n/navigation";
import { locales, type Locale } from "@/i18n/config";
import { useState, useRef, useEffect } from "react";

const FLAG_ICONS: Record<Locale, string> = {
  en: "/flags/us.svg",
  zh: "/flags/cn.svg",
  ja: "/flags/jp.svg",
};

export default function LocaleSwitcher({ compact = false }: { compact?: boolean }) {
  const t = useTranslations("language");
  const locale = useLocale() as Locale;
  const router = useRouter();
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, []);

  const handleSwitch = (next: Locale) => {
    router.replace(pathname, { locale: next });
    setOpen(false);
  };

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="flex cursor-pointer items-center gap-2"
      >
        <img
          src={FLAG_ICONS[locale]}
          alt=""
          className={`shrink-0 rounded object-cover ${compact ? "h-6 w-8" : "h-[18px] w-6"}`}
        />
        {!compact && <span className="font-semibold text-sm text-black">{t(locale)}</span>}
        <img
          src="/icons/arrow-up.svg"
          alt=""
          className={`shrink-0 transition-transform ${open ? "" : "rotate-180"} ${compact ? "size-3" : "size-2.5"}`}
        />
      </button>
      {open && (
        <div className="absolute right-0 top-full mt-2 z-[200] flex flex-col gap-3 rounded-xl border-2 border-black bg-[#fff2eb] p-3 shadow-lg">
          {locales.map((l) => (
            <button
              key={l}
              onClick={() => handleSwitch(l)}
              className={`flex cursor-pointer items-center gap-2 ${l === locale ? "font-bold" : ""}`}
            >
              <img
                src={FLAG_ICONS[l]}
                alt=""
                className="h-[18px] w-6 shrink-0 rounded object-cover"
              />
              <span className="whitespace-nowrap font-semibold text-sm text-black">
                {t(l)}
              </span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
