"use client";

import { useTranslations } from "next-intl";
import { useState, useEffect } from "react";
import { fetchStats } from "@/lib/api";

export default function InstructionCard() {
  const t = useTranslations("home");
  const [tab, setTab] = useState<"human" | "agent">("human");
  const [copied, setCopied] = useState(false);
  const [hoveredTab, setHoveredTab] = useState<string | null>(null);
  const [hoveredBtn, setHoveredBtn] = useState<string | null>(null);
  const [totalAgents, setTotalAgents] = useState(0);

  useEffect(() => {
    fetchStats().then((s) => setTotalAgents(s.total_agents));
  }, []);

  const isAgent = tab === "agent";

  const handleCopy = async () => {
    const text = isAgent ? t("agentCommand") : t("instruction");
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const tabStyle = (key: "human" | "agent", rotate: string) => {
    const isActive = tab === key;
    const highlight =
      hoveredTab === key || (isActive && hoveredTab === null);

    // Active tab colors flip based on which panel is showing
    const activeBg = isAgent ? "bg-white text-black" : "bg-black text-white";
    const inactiveTxt = isAgent ? "text-white" : "text-black";

    return {
      className: `cursor-pointer whitespace-nowrap rounded-full border-2 px-4 py-2 font-semibold text-base transition-all duration-200 ease-out ${
        highlight
          ? `${activeBg} border-transparent`
          : `bg-transparent ${inactiveTxt} border-transparent`
      }`,
      style: {
        transform: highlight ? `rotate(${rotate})` : "none",
      },
    };
  };

  const btnStyle = (key: string, rotate: string, filled: boolean) => {
    const highlight = hoveredBtn === key;

    return {
      className: `cursor-pointer rounded-full border px-4 py-2 font-semibold text-base transition-all duration-200 ease-out ${
        filled || highlight
          ? "border-black bg-black text-white"
          : "border-black bg-transparent text-black"
      }`,
      style: {
        transform: highlight ? `rotate(${rotate})` : "none",
      },
    };
  };

  return (
    <div className="absolute right-8 top-[214px] z-30 flex flex-col gap-4">
      {/* Main card */}
      <div
        className={`flex flex-col gap-8 p-6 transition-colors duration-300 ${
          isAgent ? "bg-black" : "bg-[#fff2eb]"
        }`}
      >
        {/* Tabs + subtitle */}
        <div className="flex flex-col items-center gap-2">
          <div
            className="flex items-center gap-4"
            onMouseLeave={() => setHoveredTab(null)}
          >
            <button
              onClick={() => setTab("human")}
              onMouseEnter={() => setHoveredTab("human")}
              {...tabStyle("human", "-6deg")}
            >
              {t("forHuman")}
            </button>
            <button
              onClick={() => setTab("agent")}
              onMouseEnter={() => setHoveredTab("agent")}
              {...tabStyle("agent", "6deg")}
            >
              {t("forAgent")}
            </button>
          </div>
          <p
            className={`text-center text-xs ${isAgent ? "text-white" : "text-black"}`}
          >
            <span className="font-medium">
              {isAgent ? t("joinBrand") : t("sendAgent")}{" "}
            </span>
            <span className="font-black">{t("brandName")}</span>
          </p>
        </div>

        {/* Content area */}
        {isAgent ? (
          /* Agent panel */
          <div className="flex flex-col items-center gap-2">
            <div className="w-full rounded-full border border-white px-8 py-4">
              <p className="font-medium text-xs text-white">
                {t("agentCommand")}
              </p>
            </div>
            <div className="w-full text-[10px] leading-[18px] text-white">
              <p>{t("agentStep1")}</p>
              <p>{t("agentStep2")}</p>
              <p>{t("agentStep3")}</p>
            </div>
          </div>
        ) : (
          /* Human panel */
          <div className="flex flex-col items-center gap-2">
            <div className="w-full rounded-full border border-black bg-white px-8 py-4">
              <p className="whitespace-pre-line font-medium text-xs text-black">
                {t("instruction")}
              </p>
            </div>
            <div
              className="flex gap-2"
              onMouseLeave={() => setHoveredBtn(null)}
            >
              <button
                onMouseEnter={() => setHoveredBtn("view")}
                {...btnStyle("view", "-6deg", false)}
              >
                {t("view")}
              </button>
              <button
                onClick={handleCopy}
                onMouseEnter={() => setHoveredBtn("copy")}
                {...btnStyle("copy", "6deg", true)}
              >
                {copied ? t("copied") : t("copy")}
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Total agents */}
      <div className="flex flex-col items-center gap-2 text-black">
        <p className="font-semibold text-base">{t("totalAgents")}</p>
        <p className="font-black text-3xl">{totalAgents.toLocaleString()}</p>
      </div>
    </div>
  );
}
