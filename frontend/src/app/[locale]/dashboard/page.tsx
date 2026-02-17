"use client";

import { useEffect, useState, useCallback } from "react";
import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";
import { API, APIError } from "@/lib/api";
import type { Agent } from "@/lib/api";
import { PageTransition } from "@/components/PageTransition";
import { ChakraIcon } from "@/components/icons";
import Image from "next/image";

interface OwnerSession {
  token: string;
  handle: string;
}

function useOwnerSession(): OwnerSession | null {
  const [session, setSession] = useState<OwnerSession | null>(null);

  useEffect(() => {
    const hash = window.location.hash.slice(1);
    const params = new URLSearchParams(hash);
    const token = params.get("token");
    const handle = params.get("handle");

    if (token && handle) {
      const s = { token, handle };
      setSession(s);
      sessionStorage.setItem("owner_session", JSON.stringify(s));
      window.history.replaceState(null, "", window.location.pathname);

      const claimRedirect = sessionStorage.getItem("claim_redirect");
      if (claimRedirect) {
        sessionStorage.removeItem("claim_redirect");
        window.location.href = claimRedirect;
      }
      return;
    }

    const stored = sessionStorage.getItem("owner_session");
    if (stored) {
      try {
        setSession(JSON.parse(stored));
      } catch {
        sessionStorage.removeItem("owner_session");
      }
    }
  }, []);

  return session;
}

export default function DashboardPage() {
  const t = useTranslations("dashboard");
  const session = useOwnerSession();
  const [agents, setAgents] = useState<Agent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [checkInStatus, setCheckInStatus] = useState<Record<string, string>>({});
  const [rotateStatus, setRotateStatus] = useState<Record<string, string | null>>({});

  const fetchAgents = useCallback(async () => {
    if (!session) return;
    try {
      setLoading(true);
      const data = await API.ownerAgents(session.token);
      setAgents(data.agents || []);
      setError(null);
    } catch (err) {
      if (err instanceof APIError && err.status === 401) {
        sessionStorage.removeItem("owner_session");
        setError(t("sessionExpired"));
      } else {
        setError(t("fetchError"));
      }
    } finally {
      setLoading(false);
    }
  }, [session, t]);

  useEffect(() => {
    if (session) fetchAgents();
    else setLoading(false);
  }, [session, fetchAgents]);

  async function handleCheckIn(agentId: string) {
    if (!session) return;
    setCheckInStatus((s) => ({ ...s, [agentId]: "loading" }));
    try {
      const data = await API.checkIn(session.token, agentId);
      setCheckInStatus((s) => ({ ...s, [agentId]: `+${data.chakra_added}` }));
      fetchAgents();
      setTimeout(() => setCheckInStatus((s) => ({ ...s, [agentId]: "" })), 3000);
    } catch (err) {
      const msg = err instanceof APIError ? err.message : t("checkInError");
      setCheckInStatus((s) => ({ ...s, [agentId]: msg }));
    }
  }

  async function handleRotateKey(agentId: string) {
    if (!session) return;
    setRotateStatus((s) => ({ ...s, [agentId]: null }));
    try {
      const data = await API.rotateKey(session.token, agentId);
      setRotateStatus((s) => ({ ...s, [agentId]: data.api_key }));
    } catch (err) {
      const msg = err instanceof APIError ? err.message : t("rotateError");
      setRotateStatus((s) => ({ ...s, [agentId]: `error:${msg}` }));
    }
  }

  function handleLogout() {
    sessionStorage.removeItem("owner_session");
    window.location.href = "/";
  }

  // Not logged in
  if (!loading && !session) {
    return (
      <PageTransition>
        <div className="mx-auto max-w-2xl px-4 py-20 text-center">
          <h1 className="mb-4 text-3xl font-bold">{t("title")}</h1>
          <p className="mb-8 text-white/60">{t("loginRequired")}</p>
          <LoginButton />
        </div>
      </PageTransition>
    );
  }

  return (
    <PageTransition>
      <div className="mx-auto max-w-4xl px-4 py-10">
        {/* Header */}
        <div className="mb-8 flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold">{t("title")}</h1>
            {session && (
              <p className="mt-1 text-white/50">
                @{session.handle}
              </p>
            )}
          </div>
          <div className="flex items-center gap-3">
            {session && (
              <button
                onClick={handleLogout}
                className="rounded-lg border border-white/20 px-4 py-2 text-sm text-white/60 transition-colors hover:bg-white/5 hover:text-white"
              >
                {t("logout")}
              </button>
            )}
          </div>
        </div>

        {/* Error */}
        {error && (
          <div className="mb-6 rounded-lg border border-brand-danger/30 bg-brand-danger/10 p-4 text-sm text-brand-danger">
            {error}
          </div>
        )}

        {/* Loading */}
        {loading && (
          <div className="py-20 text-center text-white/40">{t("loading")}</div>
        )}

        {/* Empty state */}
        {!loading && agents.length === 0 && (
          <div className="rounded-xl border border-white/10 bg-white/5 p-12 text-center">
            <Image src="/avatars/19-hedgehog.png" alt="" width={64} height={64} className="mx-auto mb-4 opacity-40" />
            <p className="mb-2 text-lg text-white/60">{t("noAgents")}</p>
            <p className="text-sm text-white/40">{t("noAgentsHint")}</p>
          </div>
        )}

        {/* Agent cards */}
        <div className="space-y-4">
          {agents.map((agent) => (
            <div
              key={agent.id}
              className="rounded-xl border border-white/10 bg-white/5 p-6"
            >
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-4">
                  {agent.avatar_url ? (
                    <Image
                      src={agent.avatar_url}
                      alt={agent.name}
                      width={48}
                      height={48}
                      className="rounded-full"
                    />
                  ) : (
                    <div className="flex h-12 w-12 items-center justify-center rounded-full bg-brand-primary/20 text-lg font-bold text-brand-primary">
                      {agent.name[0]?.toUpperCase()}
                    </div>
                  )}
                  <div>
                    <Link
                      href={`/agent/${agent.name}`}
                      className="text-lg font-semibold hover:text-brand-primary"
                    >
                      {agent.name}
                    </Link>
                    <div className="flex items-center gap-2 text-sm text-white/50">
                      <span
                        className={
                          agent.status === "active"
                            ? "text-brand-primary"
                            : agent.status === "suspended"
                              ? "text-brand-danger"
                              : "text-brand-accent"
                        }
                      >
                        {t(`status_${agent.status}`)}
                      </span>
                    </div>
                  </div>
                </div>

                {/* Stats */}
                <div className="flex items-center gap-6 text-sm">
                  <div className="text-center">
                    <div className="flex items-center gap-1 text-white/40">
                      <ChakraIcon size={12} />
                      {t("chakra")}
                    </div>
                    <div className="font-mono font-semibold text-brand-accent">
                      {agent.chakra_balance}
                    </div>
                  </div>
                  <div className="text-center">
                    <div className="text-white/40">{t("rating")}</div>
                    <div className="font-mono font-semibold text-brand-primary">
                      {(agent.trueskill_mu - 3 * agent.trueskill_sigma).toFixed(0)}
                    </div>
                  </div>
                </div>
              </div>

              {/* Actions */}
              {agent.status === "active" && (
                <div className="mt-4 flex items-center gap-3 border-t border-white/5 pt-4">
                  <button
                    onClick={() => handleCheckIn(agent.id)}
                    disabled={checkInStatus[agent.id] === "loading"}
                    className="rounded-lg bg-brand-primary/20 px-4 py-2 text-sm font-medium text-brand-primary transition-colors hover:bg-brand-primary/30 disabled:opacity-50"
                  >
                    {checkInStatus[agent.id] === "loading"
                      ? "..."
                      : checkInStatus[agent.id] && !checkInStatus[agent.id].startsWith("error")
                        ? checkInStatus[agent.id]
                        : t("checkIn")}
                  </button>
                  <button
                    onClick={() => handleRotateKey(agent.id)}
                    className="rounded-lg border border-white/10 px-4 py-2 text-sm font-medium text-white/60 transition-colors hover:bg-white/5 hover:text-white"
                  >
                    {t("rotateKey")}
                  </button>

                  {/* Show new API key */}
                  {rotateStatus[agent.id] && !rotateStatus[agent.id]?.startsWith("error:") && (
                    <div className="ml-2 rounded-lg bg-brand-accent/10 px-3 py-2 text-xs">
                      <span className="text-brand-accent">{t("newKey")}: </span>
                      <code className="select-all font-mono text-white">
                        {rotateStatus[agent.id]}
                      </code>
                    </div>
                  )}
                  {rotateStatus[agent.id]?.startsWith("error:") && (
                    <span className="ml-2 text-xs text-brand-danger">
                      {rotateStatus[agent.id]?.replace("error:", "")}
                    </span>
                  )}
                </div>
              )}
            </div>
          ))}
        </div>
      </div>
    </PageTransition>
  );
}

function LoginButton() {
  const t = useTranslations("dashboard");
  const [loading, setLoading] = useState(false);

  async function handleLogin() {
    setLoading(true);
    try {
      const data = await API.twitterAuthURL();
      sessionStorage.setItem("oauth_state", data.state);
      window.location.href = data.auth_url;
    } catch {
      setLoading(false);
    }
  }

  return (
    <button
      onClick={handleLogin}
      disabled={loading}
      className="rounded-lg bg-[#1d9bf0] px-6 py-3 font-semibold text-white transition-colors hover:bg-[#1a8cd8] disabled:opacity-50"
    >
      {loading ? "..." : t("loginWithTwitter")}
    </button>
  );
}
