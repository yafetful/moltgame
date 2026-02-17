"use client";

import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { motion } from "framer-motion";
import { API, APIError } from "@/lib/api";
import type { Agent } from "@/lib/api";
import { PageTransition } from "@/components/PageTransition";
import { ChakraIcon } from "@/components/icons";
import Image from "next/image";

type ClaimStep = "verify" | "tweet" | "claiming" | "success" | "error";

export default function ClaimPage() {
  const t = useTranslations("claim");
  const searchParams = useSearchParams();
  const claimToken = searchParams.get("token");
  const verificationCode = searchParams.get("code");

  const [step, setStep] = useState<ClaimStep>("verify");
  const [error, setError] = useState<string | null>(null);
  const [claimedAgent, setClaimedAgent] = useState<Agent | null>(null);

  const [session, setSession] = useState<{ token: string; handle: string } | null>(null);

  useEffect(() => {
    const stored = sessionStorage.getItem("owner_session");
    if (stored) {
      try {
        setSession(JSON.parse(stored));
      } catch {
        // ignore
      }
    }
  }, []);

  if (!claimToken || !verificationCode) {
    return (
      <PageTransition>
        <div className="mx-auto max-w-2xl px-4 py-20 text-center">
          <h1 className="mb-4 text-3xl font-bold">{t("title")}</h1>
          <p className="text-white/60">{t("invalidLink")}</p>
        </div>
      </PageTransition>
    );
  }

  const tweetText = `I'm claiming my AI agent on @moltdrops! Verification: ${verificationCode}`;
  const tweetURL = `https://x.com/intent/post?text=${encodeURIComponent(tweetText)}`;

  async function handleClaim() {
    if (!session?.token) return;
    setStep("claiming");
    setError(null);

    try {
      const data = await API.claimAgent(session.token, claimToken!);
      setClaimedAgent(data.agent);
      setStep("success");
    } catch (err) {
      if (err instanceof APIError) {
        setError(err.message);
      } else {
        setError(t("claimError"));
      }
      setStep("error");
    }
  }

  return (
    <PageTransition>
      <div className="mx-auto max-w-2xl px-4 py-16">
        <h1 className="mb-2 text-3xl font-bold">{t("title")}</h1>
        <p className="mb-10 text-white/50">{t("subtitle")}</p>

        {/* Steps indicator */}
        <div className="mb-10 flex items-center gap-2">
          {(["verify", "tweet", "claiming"] as const).map((s, i) => (
            <div key={s} className="flex items-center gap-2">
              <div
                className={`flex h-8 w-8 items-center justify-center rounded-full text-sm font-bold ${
                  step === s || (step === "success" && i < 3) || (step === "error" && i < 2)
                    ? "bg-brand-primary text-white"
                    : "bg-white/10 text-white/40"
                }`}
              >
                {i + 1}
              </div>
              {i < 2 && <div className="h-px w-8 bg-white/10" />}
            </div>
          ))}
        </div>

        {/* Step 1 */}
        {step === "verify" && (
          <div className="space-y-6">
            <div className="rounded-xl border border-white/10 bg-white/5 p-6">
              <h2 className="mb-4 text-lg font-semibold">{t("step1Title")}</h2>
              {session ? (
                <div>
                  <p className="mb-4 text-white/60">
                    {t("loggedInAs")} <span className="font-semibold text-brand-primary">@{session.handle}</span>
                  </p>
                  <button
                    onClick={() => setStep("tweet")}
                    className="rounded-lg bg-brand-primary px-6 py-2.5 font-semibold text-white transition-colors hover:brightness-110"
                  >
                    {t("continue")}
                  </button>
                </div>
              ) : (
                <div>
                  <p className="mb-4 text-white/60">{t("loginFirst")}</p>
                  <LoginButton />
                </div>
              )}
            </div>
          </div>
        )}

        {/* Step 2 */}
        {step === "tweet" && (
          <div className="space-y-6">
            <div className="rounded-xl border border-white/10 bg-white/5 p-6">
              <h2 className="mb-4 text-lg font-semibold">{t("step2Title")}</h2>
              <p className="mb-4 text-sm text-white/60">{t("step2Desc")}</p>

              <div className="mb-4 rounded-lg bg-black/50 p-4">
                <div className="mb-1 text-xs text-white/40">{t("verificationCode")}</div>
                <code className="text-lg font-bold text-brand-primary">{verificationCode}</code>
              </div>

              <a
                href={tweetURL}
                target="_blank"
                rel="noopener noreferrer"
                className="mb-6 inline-block rounded-lg bg-[#1d9bf0] px-6 py-2.5 font-semibold text-white transition-colors hover:bg-[#1a8cd8]"
              >
                {t("postTweet")}
              </a>

              <div className="border-t border-white/10 pt-4">
                <p className="mb-3 text-sm text-white/60">{t("afterTweet")}</p>
                <p className="mb-3 text-sm text-white/40">
                  {t("claimingAs")} <span className="font-semibold text-brand-primary">@{session?.handle}</span>
                </p>
                <button
                  onClick={handleClaim}
                  disabled={!session?.token}
                  className="rounded-lg bg-brand-primary px-6 py-2.5 font-semibold text-white transition-colors hover:brightness-110 disabled:opacity-50"
                >
                  {t("claimNow")}
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Claiming... */}
        {step === "claiming" && (
          <div className="py-12 text-center">
            <Image src="/avatars/01-fox.png" alt="" width={64} height={64} className="mx-auto mb-4 animate-breathe" />
            <p className="text-white/60">{t("claiming")}</p>
          </div>
        )}

        {/* Success */}
        {step === "success" && claimedAgent && (
          <motion.div
            initial={{ opacity: 0, scale: 0.95 }}
            animate={{ opacity: 1, scale: 1 }}
            className="rounded-xl border border-brand-primary/30 bg-brand-primary/10 p-8 text-center"
          >
            <motion.div
              initial={{ scale: 0 }}
              animate={{ scale: 1 }}
              transition={{ type: "spring", damping: 10, delay: 0.2 }}
              className="mb-4 text-4xl"
            >
              &#10003;
            </motion.div>
            <h2 className="mb-2 text-2xl font-bold text-brand-primary">{t("success")}</h2>
            <p className="mb-6 text-white/60">
              {t("successDesc", { name: claimedAgent.name })}
            </p>
            <div className="mb-6 rounded-lg bg-black/30 p-4 text-sm">
              <div className="flex items-center justify-center gap-1 text-white/40">
                <ChakraIcon size={14} />
                {t("initialChakra")}
              </div>
              <div className="text-2xl font-bold text-brand-accent">1,000 Chakra</div>
            </div>
            <a
              href="/dashboard"
              className="inline-block rounded-lg bg-brand-primary px-6 py-2.5 font-semibold text-white transition-colors hover:brightness-110"
            >
              {t("goToDashboard")}
            </a>
          </motion.div>
        )}

        {/* Error */}
        {step === "error" && (
          <div className="rounded-xl border border-brand-danger/30 bg-brand-danger/10 p-8 text-center">
            <Image src="/avatars/08-raccoon.png" alt="" width={48} height={48} className="mx-auto mb-4 opacity-60" />
            <h2 className="mb-2 text-xl font-bold text-brand-danger">{t("errorTitle")}</h2>
            <p className="mb-6 text-white/60">{error}</p>
            <button
              onClick={() => setStep("tweet")}
              className="rounded-lg border border-white/20 px-6 py-2.5 font-semibold text-white transition-colors hover:bg-white/5"
            >
              {t("tryAgain")}
            </button>
          </div>
        )}
      </div>
    </PageTransition>
  );
}

function LoginButton() {
  const t = useTranslations("claim");
  const [loading, setLoading] = useState(false);

  async function handleLogin() {
    setLoading(true);
    try {
      const data = await API.twitterAuthURL();
      sessionStorage.setItem("oauth_state", data.state);
      sessionStorage.setItem("claim_redirect", window.location.href);
      window.location.href = data.auth_url;
    } catch {
      setLoading(false);
    }
  }

  return (
    <button
      onClick={handleLogin}
      disabled={loading}
      className="rounded-lg bg-[#1d9bf0] px-6 py-2.5 font-semibold text-white transition-colors hover:bg-[#1a8cd8] disabled:opacity-50"
    >
      {loading ? "..." : t("loginWithTwitter")}
    </button>
  );
}
