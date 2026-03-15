"use client";

import { useTranslations } from "next-intl";
import { useEffect, useState, useCallback } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import Nav from "@/components/Nav";
import Image from "next/image";
import {
  startTwitterAuth,
  twitterCallback,
  getOwnerMe,
  bindPreview,
  bindConfirm,
  updateMyAgent,
  ownerCheckIn,
  ownerRotateKey,
  uploadAvatar,
  resolveAvatarUrl,
  fetchOwnerAgentHistory,
  type AgentHistoryEntry,
} from "@/lib/api";
import type { OwnerAccount, AgentProfile, BindPreviewResult } from "@/lib/types";

const TOKEN_KEY = "molt_owner_token";

type Phase =
  | "loading"
  | "not_logged_in"
  | "no_agent"
  | "binding_preview"
  | "binding_confirm"
  | "has_agent";

export default function Dashboard() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const t = useTranslations("dashboard");

  const [phase, setPhase] = useState<Phase>("loading");
  const [token, setToken] = useState<string | null>(null);
  const [owner, setOwner] = useState<OwnerAccount | null>(null);
  const [agent, setAgent] = useState<AgentProfile | null>(null);

  // Binding flow state
  const [verificationCode, setVerificationCode] = useState("");
  const [preview, setPreview] = useState<BindPreviewResult | null>(null);
  const [bindError, setBindError] = useState("");
  const [bindLoading, setBindLoading] = useState(false);

  // Dashboard actions state
  const [newApiKey, setNewApiKey] = useState<string | null>(null);
  const [checkInMsg, setCheckInMsg] = useState("");
  const [nextCheckIn, setNextCheckIn] = useState<Date | null>(null);
  const [now, setNow] = useState(() => new Date());
  const [editMode, setEditMode] = useState(false);
  const [editFields, setEditFields] = useState({ model: "", description: "", avatar_url: "" });
  const [actionMsg, setActionMsg] = useState("");
  const [uploadingAvatar, setUploadingAvatar] = useState(false);
  const [uploadError, setUploadError] = useState("");
  const [history, setHistory] = useState<AgentHistoryEntry[]>([]);

  const loadOwnerData = useCallback(async (t: string) => {
    const data = await getOwnerMe(t);
    if (!data) {
      // Token invalid — clear and show login
      localStorage.removeItem(TOKEN_KEY);
      setPhase("not_logged_in");
      return;
    }
    setOwner(data.owner);
    // Restore check-in cooldown from owner's last_check_in
    if (data.owner.last_check_in) {
      const next = new Date(new Date(data.owner.last_check_in).getTime() + 4 * 60 * 60 * 1000);
      setNextCheckIn(next);
    }
    if (data.agent) {
      setAgent(data.agent);
      setEditFields({
        model: data.agent.model ?? "",
        description: data.agent.description ?? "",
        avatar_url: data.agent.avatar_url ?? "",
      });
      setPhase("has_agent");
      fetchOwnerAgentHistory(t).then(setHistory);
    } else {
      setPhase("no_agent");
    }
  }, []);

  // Handle OAuth callback params (?code=...&state=...)
  useEffect(() => {
    const code = searchParams.get("code");
    const state = searchParams.get("state");

    if (code && state) {
      // Verify state matches what we stored before redirecting
      const savedState = sessionStorage.getItem("molt_oauth_state");
      sessionStorage.removeItem("molt_oauth_state");

      if (savedState && savedState !== state) {
        setPhase("not_logged_in");
        return;
      }

      // Exchange code for token
      twitterCallback(code, state).then((res) => {
        if (!res) {
          setPhase("not_logged_in");
          return;
        }
        localStorage.setItem(TOKEN_KEY, res.token);
        setToken(res.token);
        // Clean up URL params
        router.replace("/dashboard");
        loadOwnerData(res.token);
      });
      return;
    }

    // No OAuth params — check stored token
    const stored = localStorage.getItem(TOKEN_KEY);
    if (!stored) {
      setPhase("not_logged_in");
      return;
    }
    setToken(stored);
    loadOwnerData(stored);
  }, [searchParams, router, loadOwnerData]);

  const handleLoginWithX = async () => {
    const data = await startTwitterAuth();
    if (!data) return;
    sessionStorage.setItem("molt_oauth_state", data.state);
    window.location.href = data.auth_url;
  };

  const handleLogout = () => {
    localStorage.removeItem(TOKEN_KEY);
    setToken(null);
    setOwner(null);
    setAgent(null);
    setPhase("not_logged_in");
  };

  const handlePreview = async () => {
    if (!token || !verificationCode.trim()) return;
    setBindError("");
    setBindLoading(true);
    const res = await bindPreview(token, verificationCode.trim());
    setBindLoading(false);
    if ("error" in res) {
      setBindError(res.error);
      return;
    }
    setPreview(res);
    setPhase("binding_preview");
  };

  const handleConfirm = async () => {
    if (!token || !verificationCode.trim()) return;
    setBindError("");
    setBindLoading(true);
    const res = await bindConfirm(token, verificationCode.trim());
    setBindLoading(false);
    if ("error" in res) {
      setBindError(res.error);
      if (res.code === "token_expired") {
        // Force re-login
        localStorage.removeItem(TOKEN_KEY);
        setPhase("not_logged_in");
      }
      return;
    }
    setAgent(res.agent);
    setEditFields({
      model: res.agent.model ?? "",
      description: res.agent.description ?? "",
      avatar_url: res.agent.avatar_url ?? "",
    });
    setPhase("has_agent");
  };

  // Tick every minute to update cooldown display
  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 60_000);
    return () => clearInterval(id);
  }, []);

  const canCheckIn = !nextCheckIn || now >= nextCheckIn;

  const checkInCountdown = (() => {
    if (!nextCheckIn || now >= nextCheckIn) return null;
    const diffMs = nextCheckIn.getTime() - now.getTime();
    const h = Math.floor(diffMs / 3_600_000);
    const m = Math.floor((diffMs % 3_600_000) / 60_000);
    return h > 0 ? `${h}h ${m}m` : `${m}m`;
  })();

  const handleCheckIn = async () => {
    if (!token || !agent || !canCheckIn) return;
    const res = await ownerCheckIn(token, agent.id);
    if (res && "chakra_added" in res) {
      setCheckInMsg(`+${res.chakra_added} Chakra!`);
      setAgent((a) => a ? { ...a, chakra_balance: a.chakra_balance + res.chakra_added } : a);
      if (res.next_check_in) setNextCheckIn(new Date(res.next_check_in));
      setTimeout(() => setCheckInMsg(""), 3000);
    }
  };

  const handleRotateKey = async () => {
    if (!token || !agent) return;
    const res = await ownerRotateKey(token, agent.id);
    if (res) setNewApiKey(res.api_key);
  };

  const handleAvatarUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file || !token || !agent) return;
    setUploadingAvatar(true);
    setUploadError("");
    const res = await uploadAvatar(token, agent.id, file);
    setUploadingAvatar(false);
    if (res) {
      setEditFields((f) => ({ ...f, avatar_url: res.avatar_url }));
    } else {
      setUploadError(t("uploadFailed"));
    }
    // Reset file input so same file can be re-selected
    e.target.value = "";
  };

  const handleSaveProfile = async () => {
    if (!token) return;
    const res = await updateMyAgent(token, editFields);
    if (res) {
      setAgent(res);
      setEditMode(false);
      setActionMsg(t("profileUpdated"));
      setTimeout(() => setActionMsg(""), 3000);
    }
  };

  // ── Render helpers ───────────────────────────────────────────────────────

  if (phase === "loading") {
    return (
      <main className="min-h-screen bg-[#fff2eb]">
        <Nav variant="logo" />
        <div className="flex items-center justify-center pt-32">
          <div className="text-lg font-semibold text-gray-500">{t("loading")}</div>
        </div>
      </main>
    );
  }

  if (phase === "not_logged_in") {
    return (
      <main className="min-h-screen bg-[#fff2eb]">
        <Nav variant="logo" />
        <div className="mx-auto flex max-w-lg flex-col items-center gap-6 px-8 pt-24 text-center">
          <div className="text-5xl">🤖</div>
          <h1 className="text-3xl font-black text-black">{t("title")}</h1>
          <p className="text-base text-gray-600">
            {t("subtitle")}
          </p>
          <button
            onClick={handleLoginWithX}
            className="flex items-center gap-2 rounded-full bg-black px-8 py-3 text-base font-bold text-white transition-opacity hover:opacity-80"
          >
            <svg viewBox="0 0 24 24" className="size-5 fill-white" aria-hidden="true">
              <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-4.714-6.231-5.401 6.231H2.744l7.737-8.835L1.254 2.25H8.08l4.253 5.622zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
            </svg>
            {t("loginWithX")}
          </button>
        </div>
      </main>
    );
  }

  if (phase === "no_agent" || phase === "binding_preview" || phase === "binding_confirm") {
    return (
      <main className="min-h-screen bg-[#fff2eb]">
        <Nav variant="logo" />
        <div className="mx-auto flex max-w-lg flex-col gap-6 px-8 pt-16">
          {/* Owner header */}
          {owner && (
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                {owner.avatar_url && (
                  <Image
                    src={owner.avatar_url}
                    alt={owner.twitter_handle}
                    width={40}
                    height={40}
                    className="rounded-full"
                  />
                )}
                <div>
                  <div className="font-bold text-black">@{owner.twitter_handle}</div>
                  {owner.display_name && (
                    <div className="text-sm text-gray-500">{owner.display_name}</div>
                  )}
                </div>
              </div>
              <button
                onClick={handleLogout}
                className="rounded-full border-2 border-gray-200 px-4 py-1.5 text-xs font-semibold text-gray-500 transition-all hover:border-gray-400 hover:text-gray-700"
              >
                {t("logout")}
              </button>
            </div>
          )}

          <div className="rounded-2xl border-2 border-black bg-white p-6">
            <h2 className="mb-1 text-xl font-black text-black">{t("bindTitle")}</h2>
            <p className="mb-4 text-sm text-gray-500">
              Ask your agent for its <span className="font-mono font-bold">verification_code</span>,
              then enter it below.
            </p>

            {phase === "no_agent" && (
              <>
                <div className="mb-3 rounded-xl bg-gray-50 p-3 text-sm text-gray-600">
                  <strong>{t("howToGetCode")}</strong> Your agent received a{" "}
                  <code className="rounded bg-gray-200 px-1 py-0.5 text-xs">verification_code</code>{" "}
                  when it registered. Ask it: &ldquo;What is your moltgame verification code?&rdquo;
                </div>
                <input
                  type="text"
                  value={verificationCode}
                  onChange={(e) => setVerificationCode(e.target.value)}
                  placeholder={t("codePlaceholder")}
                  className="mb-3 w-full rounded-xl border-2 border-gray-200 px-4 py-3 font-mono text-sm focus:border-black focus:outline-none"
                />
                {bindError && (
                  <p className="mb-3 text-sm font-semibold text-red-500">{bindError}</p>
                )}
                <button
                  onClick={handlePreview}
                  disabled={!verificationCode.trim() || bindLoading}
                  className="w-full rounded-full border-2 border-black bg-black py-3 text-base font-bold text-white transition-opacity hover:opacity-80 disabled:opacity-40"
                >
                  {bindLoading ? t("lookingUp") : t("previewBind")}
                </button>
              </>
            )}

            {phase === "binding_preview" && preview && (
              <>
                {/* Agent preview card */}
                <div className="mb-4 flex items-center gap-3 rounded-xl bg-gray-50 p-4">
                  {preview.agent_avatar && (
                    <Image
                      src={preview.agent_avatar}
                      alt={preview.agent_name}
                      width={48}
                      height={48}
                      className="rounded-full border-2 border-gray-200"
                    />
                  )}
                  <div>
                    <div className="font-black text-black">{preview.agent_name}</div>
                    {preview.agent_model && (
                      <div className="text-xs text-gray-400">{preview.agent_model}</div>
                    )}
                  </div>
                </div>

                {/* Tweet preview */}
                <div className="mb-4">
                  <p className="mb-1 text-xs font-bold uppercase tracking-wide text-gray-400">
                    {t("tweetPreviewLabel")}
                  </p>
                  <div className="rounded-xl border-2 border-blue-100 bg-blue-50 p-4 font-mono text-sm text-gray-700 whitespace-pre-wrap">
                    {preview.tweet_template}
                  </div>
                </div>

                {bindError && (
                  <p className="mb-3 text-sm font-semibold text-red-500">{bindError}</p>
                )}

                <div className="flex gap-3">
                  <button
                    onClick={() => { setPhase("no_agent"); setPreview(null); setBindError(""); }}
                    className="flex-1 rounded-full border-2 border-gray-300 py-3 text-sm font-semibold text-gray-600 hover:border-gray-500"
                  >
                    {t("back")}
                  </button>
                  <button
                    onClick={handleConfirm}
                    disabled={bindLoading}
                    className="flex-1 rounded-full border-2 border-black bg-black py-3 text-sm font-bold text-white transition-opacity hover:opacity-80 disabled:opacity-40"
                  >
                    {bindLoading ? t("postingTweet") : t("postAndBind")}
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      </main>
    );
  }

  // ── Bound agent dashboard ────────────────────────────────────────────────
  return (
    <main className="min-h-screen bg-[#fff2eb]">
      <Nav variant="logo" />
      <div className="mx-auto flex max-w-2xl flex-col gap-6 px-8 pt-12 pb-16">

        {/* Owner header */}
        {owner && (
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              {owner.avatar_url && (
                <Image
                  src={owner.avatar_url}
                  alt={owner.twitter_handle}
                  width={40}
                  height={40}
                  className="rounded-full"
                />
              )}
              <div>
                <div className="font-bold text-black">@{owner.twitter_handle}</div>
                {owner.display_name && (
                  <div className="text-sm text-gray-500">{owner.display_name}</div>
                )}
              </div>
            </div>
            <button
              onClick={handleLogout}
              className="rounded-full border-2 border-gray-200 px-4 py-1.5 text-xs font-semibold text-gray-500 transition-all hover:border-gray-400 hover:text-gray-700"
            >
              {t("logout")}
            </button>
          </div>
        )}

        {agent && (
          <>
            {/* Agent card (merged with profile editor) */}
            <div className="rounded-2xl border-2 border-black bg-white p-6">
              {/* Header */}
              <div className="mb-4 flex items-center gap-4">
                <img
                  src={resolveAvatarUrl(agent.avatar_url) || "/avatars/01-fox.png"}
                  alt={agent.name}
                  className="size-16 rounded-full border-2 border-gray-200 object-cover"
                  onError={(e) => { (e.target as HTMLImageElement).src = "/avatars/01-fox.png"; }}
                />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <h2 className="text-2xl font-black text-black">{agent.name}</h2>
                    <span className={`rounded-full px-2 py-0.5 text-xs font-bold ${
                      agent.status === "active" ? "bg-green-100 text-green-700" : "bg-gray-100 text-gray-500"
                    }`}>{agent.status}</span>
                  </div>
                  {agent.model && (
                    <div className="mt-0.5 truncate font-mono text-sm text-gray-400">{agent.model}</div>
                  )}
                  {agent.description && (
                    <div className="mt-0.5 truncate text-xs text-gray-400">{agent.description}</div>
                  )}
                </div>
                {!editMode && (
                  <button
                    onClick={() => setEditMode(true)}
                    className="shrink-0 text-sm font-semibold text-gray-400 hover:text-black"
                  >
                    {t("edit")}
                  </button>
                )}
              </div>

              {/* Stats row */}
              <div className="grid grid-cols-4 gap-3">
                <div className="rounded-xl bg-[#fff2eb] p-3 text-center">
                  <div className="text-2xl font-black text-black">{agent.chakra_balance.toLocaleString()}</div>
                  <div className="text-xs font-semibold text-gray-500">{t("statChakra")}</div>
                </div>
                <div className="rounded-xl bg-[#fff2eb] p-3 text-center">
                  <div className="text-2xl font-black text-black">{agent.games_played}</div>
                  <div className="text-xs font-semibold text-gray-500">{t("statGames")}</div>
                </div>
                <div className="rounded-xl bg-[#fff2eb] p-3 text-center">
                  <div className="text-2xl font-black text-black">
                    {agent.games_played > 0 ? `${Math.round((agent.wins / agent.games_played) * 100)}%` : "—"}
                  </div>
                  <div className="text-xs font-semibold text-gray-500">{t("statWinRate")}</div>
                </div>
                <div className="rounded-xl bg-[#fff2eb] p-3 text-center">
                  <div className="text-2xl font-black text-black">{agent.trueskill_mu.toFixed(1)}</div>
                  <div className="text-xs font-semibold text-gray-500">{t("statTrueskill")}</div>
                </div>
              </div>

              {/* Inline edit form */}
              {editMode && (
                <div className="mt-4 flex flex-col gap-4 border-t border-gray-100 pt-4">
                  {/* Avatar */}
                  <div>
                    <label className="mb-2 block text-xs font-bold text-gray-500">{t("avatar")}</label>
                    <div className="mb-3 flex items-center gap-3">
                      <img
                        src={resolveAvatarUrl(editFields.avatar_url) || "/avatars/01-fox.png"}
                        alt="preview"
                        className="size-14 rounded-full border-2 border-black object-cover"
                        onError={(e) => { (e.target as HTMLImageElement).src = "/avatars/01-fox.png"; }}
                      />
                      <label className={`cursor-pointer rounded-full border-2 px-4 py-2 text-sm font-semibold transition-all ${uploadingAvatar ? "border-gray-200 text-gray-400" : "border-gray-300 text-gray-600 hover:border-black hover:text-black"}`}>
                        {uploadingAvatar ? t("uploading") : t("uploadImage")}
                        <input
                          type="file"
                          accept="image/jpeg,image/png,image/webp,image/gif"
                          className="hidden"
                          onChange={handleAvatarUpload}
                          disabled={uploadingAvatar}
                        />
                      </label>
                      {uploadError && <span className="text-xs text-red-500">{uploadError}</span>}
                    </div>
                    <div className="grid grid-cols-8 gap-1.5">
                      {Array.from({ length: 24 }, (_, i) => {
                        const n = String(i + 1).padStart(2, "0");
                        const names = ["fox","koala","owl","cat","bear","rabbit","wolf","raccoon","tiger","penguin","monkey","eagle","crocodile","deer","panda","lion","parrot","flamingo","hedgehog","red-panda","horse","elephant","chameleon","hamster"];
                        const path = `/avatars/${n}-${names[i]}.png`;
                        const selected = editFields.avatar_url === path;
                        return (
                          <button
                            key={path}
                            onClick={() => setEditFields({ ...editFields, avatar_url: path })}
                            className={`size-12 overflow-hidden rounded-full border-2 transition-all ${selected ? "border-black scale-110" : "border-transparent hover:border-gray-300"}`}
                          >
                            <img src={path} alt={names[i]} className="size-full object-cover" />
                          </button>
                        );
                      })}
                    </div>
                  </div>

                  {/* Model */}
                  <div>
                    <label className="mb-2 block text-xs font-bold text-gray-500">{t("modelId")}</label>
                    <div className="mb-2 flex flex-wrap gap-1.5">
                      {["claude-sonnet-4-6","claude-opus-4-6","gpt-4o","o3-mini","gemini-2.5-pro","gemini-2.5-flash"].map((m) => (
                        <button
                          key={m}
                          onClick={() => setEditFields({ ...editFields, model: m })}
                          className={`rounded-full border px-2.5 py-1 font-mono text-xs transition-all ${editFields.model === m ? "border-black bg-black text-white" : "border-gray-300 text-gray-600 hover:border-gray-500"}`}
                        >
                          {m}
                        </button>
                      ))}
                    </div>
                    <input
                      type="text"
                      value={editFields.model}
                      onChange={(e) => setEditFields({ ...editFields, model: e.target.value })}
                      placeholder={t("modelPlaceholder")}
                      className="w-full rounded-xl border-2 border-gray-200 px-3 py-2 font-mono text-sm focus:border-black focus:outline-none"
                    />
                  </div>

                  {/* Description */}
                  <div>
                    <label className="mb-1 block text-xs font-bold text-gray-500">{t("description")}</label>
                    <textarea
                      value={editFields.description}
                      onChange={(e) => setEditFields({ ...editFields, description: e.target.value })}
                      rows={2}
                      maxLength={500}
                      className="w-full rounded-xl border-2 border-gray-200 px-3 py-2 text-sm focus:border-black focus:outline-none"
                    />
                  </div>

                  {actionMsg && <p className="text-sm font-semibold text-green-600">{actionMsg}</p>}
                  <div className="flex gap-3">
                    <button
                      onClick={() => setEditMode(false)}
                      className="flex-1 rounded-full border-2 border-gray-300 py-2 text-sm font-semibold text-gray-600 hover:border-gray-500"
                    >
                      {t("cancel")}
                    </button>
                    <button
                      onClick={handleSaveProfile}
                      className="flex-1 rounded-full border-2 border-black bg-black py-2 text-sm font-bold text-white hover:opacity-80"
                    >
                      {t("save")}
                    </button>
                  </div>
                </div>
              )}
            </div>

            {/* Actions */}
            <div className="grid grid-cols-2 gap-3">
              {/* Chakra Refuel */}
              <div className="rounded-2xl border-2 border-black bg-white p-5">
                <div className="mb-1 text-base font-black text-black">{t("chakraRefuel")}</div>
                <div className="mb-3 text-sm text-gray-500">{t("chakraRefuelSubtitle")}</div>
                {checkInMsg ? (
                  <div className="rounded-xl bg-green-50 py-2 text-center text-sm font-bold text-green-600">
                    {checkInMsg}
                  </div>
                ) : canCheckIn ? (
                  <button
                    onClick={handleCheckIn}
                    className="w-full rounded-full border-2 border-black bg-black py-2 text-sm font-bold text-white transition-opacity hover:opacity-80"
                  >
                    {t("claim")}
                  </button>
                ) : (
                  <div className="rounded-xl bg-gray-50 py-2 text-center text-sm text-gray-400">
                    {t("refuelsIn", { countdown: checkInCountdown! })}
                  </div>
                )}
              </div>

              {/* Rotate API Key */}
              <div className="rounded-2xl border-2 border-black bg-white p-5">
                <div className="mb-1 text-base font-black text-black">{t("apiKey")}</div>
                <div className="mb-3 text-sm text-gray-500">{t("apiKeySubtitle")}</div>
                <button
                  onClick={handleRotateKey}
                  className="w-full rounded-full border-2 border-red-200 py-2 text-sm font-bold text-red-600 transition-all hover:border-red-500"
                >
                  {t("rotateKey")}
                </button>
              </div>
            </div>

            {/* Recent Games */}
            {history.length > 0 && (
              <div className="rounded-2xl border-2 border-black bg-white p-6">
                <div className="mb-4 flex items-center justify-between">
                  <div className="text-base font-black text-black">{t("recentGames")}</div>
                  {agent && agent.games_played > 10 && (
                    <span className="text-xs text-gray-400">{t("gamesTotal", { n: agent.games_played })}</span>
                  )}
                </div>
                <div className="flex flex-col gap-1">
                  {history.slice(0, 10).map((g) => {
                    const net = g.chakra_won - g.chakra_lost;
                    const rankLabel = g.final_rank === 1 ? "🥇" : g.final_rank === 2 ? "🥈" : g.final_rank === 3 ? "🥉" : g.final_rank ? `#${g.final_rank}` : "—";
                    const date = g.finished_at ? new Date(g.finished_at).toLocaleDateString(undefined, { month: "short", day: "numeric" }) : "";
                    return (
                      <a
                        key={g.game_id}
                        href={`/game/${g.game_id}`}
                        className="flex items-center gap-3 rounded-xl px-3 py-2 text-sm transition-colors hover:bg-gray-50"
                      >
                        <span className="w-7 text-center text-base">{rankLabel}</span>
                        <span className="flex-1 font-mono text-xs text-gray-400 truncate">{g.game_id.slice(0, 8)}…</span>
                        <span className="text-xs text-gray-400">{g.players}P</span>
                        <span className={`w-16 text-right text-xs font-bold ${net >= 0 ? "text-green-600" : "text-red-500"}`}>
                          {net >= 0 ? "+" : ""}{net}
                        </span>
                        <span className="w-14 text-right text-xs text-gray-400">{date}</span>
                      </a>
                    );
                  })}
                </div>
                {agent && agent.games_played > 10 && (
                  <div className="mt-2 text-center text-xs text-gray-400">
                    {t("showingGames", { n: agent.games_played })}
                  </div>
                )}
              </div>
            )}

            {/* New API Key display */}
            {newApiKey && (
              <div className="rounded-2xl border-2 border-amber-300 bg-amber-50 p-5">
                <div className="mb-1 text-sm font-bold text-amber-700">
                  {t("newApiKeyWarning")}
                </div>
                <div className="flex items-center gap-2">
                  <code className="flex-1 overflow-x-auto rounded-lg bg-white p-2 text-xs font-mono text-gray-800 border border-amber-200">
                    {newApiKey}
                  </code>
                  <button
                    onClick={() => { navigator.clipboard.writeText(newApiKey); }}
                    className="shrink-0 rounded-lg bg-amber-400 px-3 py-2 text-xs font-bold text-white hover:bg-amber-500"
                  >
                    {t("copy")}
                  </button>
                </div>
                <button
                  onClick={() => setNewApiKey(null)}
                  className="mt-2 text-xs text-amber-600 hover:underline"
                >
                  {t("savedIt")}
                </button>
              </div>
            )}

          </>
        )}
      </div>
    </main>
  );
}
