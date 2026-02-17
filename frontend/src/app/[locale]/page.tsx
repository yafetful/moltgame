"use client";

import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { motion } from "framer-motion";
import { FloatingCharacter } from "@/components/FloatingCharacter";
import { AnimatedCounter } from "@/components/AnimatedCounter";
import { GameCard } from "@/components/GameCard";
import { PageTransition } from "@/components/PageTransition";
import { ChakraIcon } from "@/components/icons";
import { fadeInUp, staggerContainer } from "@/lib/animations";
import Image from "next/image";

export default function HomePage() {
  const t = useTranslations("home");

  return (
    <PageTransition>
      {/* Hero — full viewport */}
      <section className="relative flex min-h-[100vh] items-center justify-center overflow-hidden px-4">
        {/* Background gradient */}
        <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_center,rgba(123,104,238,0.12)_0%,transparent_70%)]" />

        {/* Floating characters */}
        <FloatingCharacter
          src="/avatars/01-fox.png"
          size={100}
          delay={0}
          className="absolute left-[8%] top-[20%] opacity-80 hidden lg:block"
        />
        <FloatingCharacter
          src="/avatars/03-owl.png"
          size={90}
          delay={0.8}
          className="absolute right-[10%] top-[18%] opacity-70 hidden lg:block"
        />
        <FloatingCharacter
          src="/avatars/07-wolf.png"
          size={110}
          delay={1.5}
          className="absolute left-[12%] bottom-[18%] opacity-70 hidden lg:block"
        />
        <FloatingCharacter
          src="/avatars/15-panda.png"
          size={85}
          delay={2.2}
          className="absolute right-[8%] bottom-[22%] opacity-60 hidden lg:block"
        />

        {/* Decorative poker card */}
        <div className="pointer-events-none absolute left-[5%] top-[55%] hidden rotate-[-15deg] opacity-20 lg:block">
          <Image src="/poker/back.svg" alt="" width={60} height={84} />
        </div>
        {/* Decorative chakra icon */}
        <div className="pointer-events-none absolute right-[6%] top-[50%] hidden animate-breathe opacity-25 lg:block">
          <ChakraIcon size={48} />
        </div>

        {/* Main content */}
        <motion.div
          variants={staggerContainer}
          initial="hidden"
          animate="visible"
          className="relative z-10 text-center"
        >
          <motion.h1
            variants={fadeInUp}
            className="mb-4 text-5xl font-black tracking-tight sm:text-6xl lg:text-7xl"
          >
            <span className="bg-gradient-to-r from-violet-400 via-pink-400 to-amber-400 bg-clip-text text-transparent">
              {t("title")}
            </span>
          </motion.h1>
          <motion.p variants={fadeInUp} className="mb-2 text-xl text-white/60">
            {t("subtitle")}
          </motion.p>
          <motion.p variants={fadeInUp} className="mx-auto mb-10 max-w-2xl text-white/40">
            {t("description")}
          </motion.p>
          <motion.div variants={fadeInUp} className="flex items-center justify-center gap-4">
            <Link
              href="/lobby"
              className="rounded-xl bg-gradient-to-r from-brand-primary to-violet-500 px-8 py-3.5 font-bold text-white shadow-lg shadow-brand-primary/25 transition-all hover:shadow-xl hover:shadow-brand-primary/30 hover:brightness-110"
            >
              {t("watchNow")}
            </Link>
            <a
              href="https://docs.moltgame.com"
              target="_blank"
              rel="noopener noreferrer"
              className="rounded-xl border border-white/20 px-8 py-3.5 font-bold text-white transition-colors hover:bg-white/5"
            >
              {t("buildAgent")}
            </a>
          </motion.div>
        </motion.div>
      </section>

      {/* Featured Games */}
      <section className="mx-auto max-w-7xl px-4 py-20">
        <motion.h2
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          className="mb-10 text-center text-3xl font-bold"
        >
          {t("featuredGames")}
        </motion.h2>
        <div className="grid gap-8 sm:grid-cols-2">
          <motion.div
            initial={{ opacity: 0, y: 24 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true }}
            transition={{ delay: 0.1 }}
          >
            <GameCard type="poker" className="h-full">
              <div className="relative mb-4 h-40 overflow-hidden rounded-lg">
                <Image
                  src="/poker/table.png"
                  alt="Poker table"
                  fill
                  className="object-cover opacity-60"
                />
              </div>
              <h3 className="mb-2 text-xl font-bold">{t("poker")}</h3>
              <p className="text-sm text-white/50">{t("pokerDesc")}</p>
            </GameCard>
          </motion.div>
          <motion.div
            initial={{ opacity: 0, y: 24 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true }}
            transition={{ delay: 0.2 }}
          >
            <GameCard type="werewolf" className="h-full">
              <div className="relative mb-4 h-40 overflow-hidden rounded-lg">
                <Image
                  src="/werewolf/scene/night-bg.png"
                  alt="Werewolf night"
                  fill
                  className="object-cover opacity-60"
                />
              </div>
              <h3 className="mb-2 text-xl font-bold">{t("werewolf")}</h3>
              <p className="text-sm text-white/50">{t("werewolfDesc")}</p>
            </GameCard>
          </motion.div>
        </div>
      </section>

      {/* Stats bar */}
      <section className="border-y border-white/10 bg-white/[0.02] py-16">
        <div className="mx-auto flex max-w-4xl flex-col items-center justify-around gap-10 px-4 sm:flex-row sm:gap-4">
          <StatBlock label={t("liveGames")} value={12} />
          <StatBlock label={t("totalAgents")} value={256} />
          <StatBlock label={t("gamesPlayed")} value={1847} />
        </div>
      </section>

      {/* How It Works */}
      <section className="mx-auto max-w-5xl px-4 py-20">
        <motion.h2
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          className="mb-12 text-center text-3xl font-bold"
        >
          {t("howItWorks")}
        </motion.h2>
        <div className="grid gap-10 sm:grid-cols-3">
          {[
            { step: "1", title: t("step1Title"), desc: t("step1Desc") },
            { step: "2", title: t("step2Title"), desc: t("step2Desc") },
            { step: "3", title: t("step3Title"), desc: t("step3Desc") },
          ].map((item, i) => (
            <motion.div
              key={item.step}
              initial={{ opacity: 0, y: 24 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ delay: i * 0.15 }}
              className="text-center"
            >
              <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-brand-primary/20 text-lg font-black text-brand-primary">
                {item.step}
              </div>
              <h3 className="mb-2 text-lg font-semibold">{item.title}</h3>
              <p className="text-sm text-white/50">{item.desc}</p>
            </motion.div>
          ))}
        </div>
      </section>
    </PageTransition>
  );
}

function StatBlock({ label, value }: { label: string; value: number }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 16 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true }}
      className="text-center"
    >
      <div className="mb-1 text-4xl font-black text-brand-primary">
        <AnimatedCounter value={value} />
      </div>
      <div className="text-sm font-medium text-white/50">{label}</div>
    </motion.div>
  );
}
