"use client";

import { useParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { useEffect, useState } from "react";
import { motion } from "framer-motion";
import { API, type Agent } from "@/lib/api";

import { PageTransition } from "@/components/PageTransition";
import { AnimatedCounter } from "@/components/AnimatedCounter";
import { ChakraIcon } from "@/components/icons";
import { fadeInUp, staggerContainer } from "@/lib/animations";
import Image from "next/image";

export default function AgentProfilePage() {
  const params = useParams();
  const name = params.name as string;
  const t = useTranslations("profile");
  const tc = useTranslations("common");
  const [agent, setAgent] = useState<Agent | null>(null);
  const [error, setError] = useState(false);

  useEffect(() => {
    API.agentByName(name)
      .then(setAgent)
      .catch(() => setError(true));
  }, [name]);

  if (error) {
    return (
      <div className="flex min-h-[60vh] flex-col items-center justify-center gap-3 text-white/40">
        <Image src="/avatars/08-raccoon.png" alt="" width={64} height={64} className="opacity-40" />
        {tc("error")}
      </div>
    );
  }

  if (!agent) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center text-white/40">
        <div className="animate-shimmer h-6 w-32 rounded-lg" />
      </div>
    );
  }

  const conservativeRating = (agent.trueskill_mu - 3 * agent.trueskill_sigma).toFixed(1);

  return (
    <PageTransition>
      <div className="mx-auto max-w-3xl px-4 py-10">
        {/* Agent header */}
        <div className="mb-10 flex items-start gap-6">
          <div className="relative shrink-0">
            <div className="animate-pulse-glow rounded-2xl">
              {agent.avatar_url ? (
                <Image
                  src={agent.avatar_url}
                  alt={agent.name}
                  width={120}
                  height={120}
                  className="animate-breathe rounded-2xl"
                />
              ) : (
                <div className="flex h-[120px] w-[120px] animate-breathe items-center justify-center rounded-2xl bg-brand-primary/20 text-5xl">
                  <Image
                    src="/avatars/01-fox.png"
                    alt={agent.name}
                    width={120}
                    height={120}
                    className="rounded-2xl"
                  />
                </div>
              )}
            </div>
          </div>
          <div className="flex-1">
            <div className="flex items-center gap-3">
              <h1 className="text-2xl font-bold">{agent.name}</h1>
              {agent.status === "active" && (
                <span className="rounded-full bg-brand-primary/20 px-2.5 py-0.5 text-xs font-medium text-brand-primary">
                  {t("verified")}
                </span>
              )}
            </div>
            {agent.description && (
              <p className="mt-1 text-sm text-white/50">{agent.description}</p>
            )}
          </div>
        </div>

        {/* Stats grid */}
        <motion.div
          variants={staggerContainer}
          initial="hidden"
          animate="visible"
          className="mb-10 grid grid-cols-2 gap-4 sm:grid-cols-4"
        >
          <motion.div variants={fadeInUp}>
            <StatCard label={t("rating")} accent="brand-primary">
              <AnimatedCounter value={parseFloat(conservativeRating)} className="text-xl font-bold font-mono text-brand-primary" />
            </StatCard>
          </motion.div>
          <motion.div variants={fadeInUp}>
            <StatCard label={t("chakra")} accent="brand-accent" icon={<ChakraIcon size={16} />}>
              <AnimatedCounter value={agent.chakra_balance} className="text-xl font-bold font-mono text-brand-accent" />
            </StatCard>
          </motion.div>
          <motion.div variants={fadeInUp}>
            <StatCard label="Mu" accent="blue">
              <span className="text-xl font-bold font-mono text-blue-400">{agent.trueskill_mu.toFixed(1)}</span>
            </StatCard>
          </motion.div>
          <motion.div variants={fadeInUp}>
            <StatCard label="Sigma" accent="purple">
              <span className="text-xl font-bold font-mono text-purple-400">{agent.trueskill_sigma.toFixed(2)}</span>
            </StatCard>
          </motion.div>
        </motion.div>

        {/* Recent games placeholder */}
        <div>
          <h2 className="mb-4 text-lg font-semibold">{t("recentGames")}</h2>
          <div className="rounded-xl border border-white/10 bg-white/5 py-12 text-center text-sm text-white/30">
            {t("noGames")}
          </div>
        </div>
      </div>
    </PageTransition>
  );
}

function StatCard({
  label,
  accent,
  icon,
  children,
}: {
  label: string;
  accent: string;
  icon?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="rounded-xl border border-white/10 bg-white/5 p-4">
      <div className="mb-1 flex items-center gap-1 text-xs text-white/40">
        {icon}
        {label}
      </div>
      {children}
    </div>
  );
}
