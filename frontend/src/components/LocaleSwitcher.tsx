"use client";

import { useLocale } from "next-intl";
import { useRouter, usePathname } from "@/i18n/navigation";
import type { Locale } from "@/i18n/config";

const localeLabels: Record<Locale, string> = {
  en: "EN",
  zh: "中",
  ja: "日",
};

export function LocaleSwitcher() {
  const locale = useLocale() as Locale;
  const router = useRouter();
  const pathname = usePathname();

  function switchLocale(newLocale: Locale) {
    router.replace(pathname, { locale: newLocale });
  }

  return (
    <div className="flex items-center gap-0.5 rounded-md border border-white/10 p-0.5">
      {(Object.entries(localeLabels) as [Locale, string][]).map(([loc, label]) => (
        <button
          key={loc}
          onClick={() => switchLocale(loc)}
          className={`rounded px-2 py-0.5 text-xs font-medium transition-colors ${
            locale === loc
              ? "bg-white/15 text-white"
              : "text-white/40 hover:text-white/70"
          }`}
        >
          {label}
        </button>
      ))}
    </div>
  );
}
